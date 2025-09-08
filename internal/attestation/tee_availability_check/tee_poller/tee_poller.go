package teepoller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
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
	teeVerifier.TeeSamples = make(map[common.Address][]teetypes.TeePollerSample)
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
				sampleAllTees(ctx, teeVerifier)
			case <-ctx.Done():
				logger.Infof("TEE poller stopped: %v", ctx.Err())
				return
			}
		}
	}()
}

func sampleAllTees(ctx context.Context, teeVerifier *verifier.TeeVerifier) {
	activeTees, err := getAllActiveTeesWithRetry(ctx, teeVerifier)
	if err != nil {
		logger.Warnf("Failed to fetch active TEEs, using last cached version: %v", err)
		activeTees = getCachedActiveTees()
		if len(activeTees.TeeIDs) == 0 {
			logger.Infof("No cached TEEs available, skipping this poll")
			return
		}
	} else {
		updateActiveTees(activeTees)
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
					state, err := queryTeeInfoAndValidate(ctx, teeVerifier, t.proxyURL)
					if err != nil {
						logger.Errorf("Failed to query teeInfo %s and validate: %v", t.proxyURL, err)
					}
					teeVerifier.SamplesMu.Lock()
					samples := teeVerifier.TeeSamples[t.teeID]
					sample := teetypes.TeePollerSample{Timestamp: time.Now().UTC(), State: state}
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

func queryTeeInfoAndValidate(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyURL string) (teetypes.TeePollerSampleState, error) {
	infoResponse, err := fetchTEEInfoData(ctx, proxyURL)
	if err != nil {
		return teetypes.TeePollerSampleInvalid, err
	}
	checkInfoChallenge, err := teeVerifier.CheckInfoChallengeIsValid(ctx, infoResponse.TeeInfo.Challenge)
	if err != nil {
		return checkInfoChallenge, err
	}
	if checkInfoChallenge == teetypes.TeePollerSampleInvalid {
		return teetypes.TeePollerSampleInvalid, nil
	}
	_, err = teeVerifier.DataVerification(infoResponse)
	if err != nil {
		return teetypes.TeePollerSampleInvalid, err
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
	activeTees, err := teeVerifier.TeeMachineRegistryCaller.GetAllActiveTeeMachines(callOpts)
	if err != nil {
		return teeList{}, fmt.Errorf("getAllActiveTeeMachines: %w", err)
	}
	logger.Debugf("TEE poller got active Tees: %v", activeTees)
	return teeList{
		TeeIDs: activeTees.TeeIds,
		URLs:   activeTees.Urls,
	}, nil
}

func getAllActiveTeesWithRetry(ctx context.Context, teeVerifier *verifier.TeeVerifier) (teeList, error) {
	return utils.Retry(chainRetries, chainRetryDelay, func() (teeList, error) {
		return getAllActiveTeeMachines(ctx, teeVerifier)
	}, nil)
}

func fetchTEEInfoData(ctx context.Context, baseURL string) (teenodetypes.TeeInfoResponse, error) {
	url := fmt.Sprintf("%s/info", baseURL)
	return utils.FetchJSON[teenodetypes.TeeInfoResponse](ctx, url, fetchTimeout)
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
