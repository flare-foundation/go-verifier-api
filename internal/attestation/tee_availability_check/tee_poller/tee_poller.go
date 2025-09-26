package teepoller

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	utils "github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	teenodetype "github.com/flare-foundation/tee-node/pkg/types"
)

type TeePollerService struct {
	cancel context.CancelFunc
}

func StartTeePoller(parentCtx context.Context, verifier *verifier.TeeVerifier) *TeePollerService {
	ctx, cancel := context.WithCancel(parentCtx)
	StartPoller(ctx, verifier)
	return &TeePollerService{cancel: cancel}
}

func (s *TeePollerService) Close() error {
	s.cancel()
	return nil
}

const (
	sampleInterval     = 1 * time.Minute
	defaultWorkerCount = 10
	fetchTimeout       = 5 * time.Second
	chainRetries       = 2
	chainRetryDelay    = 500 * time.Millisecond
	teeMachineChunk    = 100
)

var (
	lastActiveTees teeList
	teesMu         sync.RWMutex
)

type task struct {
	teeID    common.Address
	proxyURL string
}

func StartPoller(ctx context.Context, teeVerifier *verifier.TeeVerifier) {
	teeVerifier.TeeSamples = make(map[common.Address][]teetype.TeePollerSample)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("TEE poller panic recovered: %v", r)
			}
		}()
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sampleAllTees(ctx, teeVerifier, getAllActiveTeesWithRetry, queryTeeInfoAndValidate)
			case <-ctx.Done():
				logger.Infof("TEE poller stopped: %v", ctx.Err())
				return
			}
		}
	}()
}

func sampleAllTees(
	ctx context.Context,
	teeVerifier *verifier.TeeVerifier,
	getTees func(ctx context.Context, teeVerifier *verifier.TeeVerifier) (teeList, error),
	queryInfoAndValidate func(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyURL string, teeID common.Address) (teetype.TeePollerSampleState, error)) {
	activeTees, err := getTees(ctx, teeVerifier)
	if err != nil {
		logger.Warnf("Failed to fetch active TEEs, using last cached version: %v", err)
		activeTees = getCachedActiveTees()
		if len(activeTees.TeeIDs) == 0 {
			logger.Infof("No cached TEEs available, skipping this poll")
			return
		}
	} else {
		updateActiveTees(activeTees)
		filterTeeSamplesToActive(teeVerifier, activeTees)
	}

	taskCh := make(chan task, len(activeTees.TeeIDs))
	var wg sync.WaitGroup
	workers := min(defaultWorkerCount, len(activeTees.TeeIDs))
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("worker panic recovered: %v", r)
				}
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case t, ok := <-taskCh:
					if !ok {
						return
					}
					state, err := queryInfoAndValidate(ctx, teeVerifier, t.proxyURL, t.teeID)
					if err != nil {
						logger.Errorf("Failed to query teeInfo %s or validate: %v", t.proxyURL, err)
					}
					teeVerifier.SamplesMu.Lock()
					samples := teeVerifier.TeeSamples[t.teeID]
					sample := teetype.TeePollerSample{Timestamp: time.Now().UTC(), State: state}
					samples = append(samples, sample)
					if len(samples) > teeVerifier.SamplesToConsider {
						samples = samples[1:]
					}
					teeVerifier.TeeSamples[t.teeID] = samples
					teeVerifier.SamplesMu.Unlock()
				}
			}
		}()
	}
	for i, teeID := range activeTees.TeeIDs {
		taskCh <- task{teeID: teeID, proxyURL: activeTees.URLs[i]}
	}
	close(taskCh)
	wg.Wait()
	teeVerifier.SamplesMu.RLock()
	logger.Debugf("TEE poller samples: %v", teeVerifier.TeeSamples)
	teeVerifier.SamplesMu.RUnlock()
}

func queryTeeInfoAndValidate(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyURL string, teeID common.Address) (teetype.TeePollerSampleState, error) {
	infoResponse, err := fetchTEEInfoData(ctx, proxyURL)
	if err != nil {
		return teetype.TeePollerSampleInvalid, err
	}
	checkInfoChallenge, err := teeVerifier.CheckInfoChallengeIsValid(ctx, infoResponse.TeeInfo.Challenge)
	if err != nil {
		return checkInfoChallenge, err
	}
	_, err = teeVerifier.DataVerification(infoResponse, teeID)
	if err != nil {
		return teetype.TeePollerSampleInvalid, err
	}
	infoData := infoResponse.TeeInfo
	state, err := teeVerifier.CheckSigningPolicies(ctx, infoData)
	if err != nil {
		return state, err
	}
	return state, nil
}

type teeList struct {
	TeeIDs []common.Address
	URLs   []string
}

func getAllActiveTeeMachines(ctx context.Context, teeVerifier *verifier.TeeVerifier) (teeList, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	callOpts := &bind.CallOpts{
		Context: ctx,
	}
	var allTeeIDs []common.Address
	var allURLs []string
	start := big.NewInt(0)
	chunk := big.NewInt(teeMachineChunk)
	for {
		tees, err := teeVerifier.TeeMachineRegistryCaller.GetAllActiveTeeMachines(callOpts, start, new(big.Int).Add(start, chunk))
		if err != nil {
			return teeList{}, fmt.Errorf("getAllActiveTeeMachines: %w", err)
		}
		allTeeIDs = append(allTeeIDs, tees.TeeIds...)
		allURLs = append(allURLs, tees.Urls...)

		retrieved := int64(len(tees.TeeIds))
		if retrieved < chunk.Int64() {
			break
		}
		start = new(big.Int).Add(start, big.NewInt(retrieved))
	}
	activeTees := teeList{
		TeeIDs: allTeeIDs,
		URLs:   allURLs,
	}
	logger.Debugf("TEE poller got active Tees: %v", activeTees)
	return activeTees, nil
}

func getAllActiveTeesWithRetry(ctx context.Context, teeVerifier *verifier.TeeVerifier) (teeList, error) {
	return utils.Retry(chainRetries, chainRetryDelay, func() (teeList, error) {
		return getAllActiveTeeMachines(ctx, teeVerifier)
	}, nil)
}

func fetchTEEInfoData(ctx context.Context, baseURL string) (teenodetype.TeeInfoResponse, error) {
	url := fmt.Sprintf("%s/info", baseURL)
	return utils.GetJSON[teenodetype.TeeInfoResponse](ctx, url, fetchTimeout)
}

func updateActiveTees(teelist teeList) {
	teesMu.Lock()
	defer teesMu.Unlock()
	lastActiveTees = teelist
}

func getCachedActiveTees() teeList {
	teesMu.RLock()
	defer teesMu.RUnlock()
	return lastActiveTees
}

func filterTeeSamplesToActive(teeVerifier *verifier.TeeVerifier, activeTees teeList) {
	activeSet := make(map[common.Address]struct{}, len(activeTees.TeeIDs))
	for _, id := range activeTees.TeeIDs {
		activeSet[id] = struct{}{}
	}

	teeVerifier.SamplesMu.Lock()
	defer teeVerifier.SamplesMu.Unlock()

	for teeID := range teeVerifier.TeeSamples {
		if _, ok := activeSet[teeID]; !ok {
			delete(teeVerifier.TeeSamples, teeID)
		}
	}
}
