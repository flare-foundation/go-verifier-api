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
	teeId    common.Address
	proxyUrl string
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
	activeTees, err := getAllActiveTeesWithRetry(teeVerifier)
	if err != nil {
		logger.Warnf("Failed to fetch active TEEs, using last cached version: %v", err)
		activeTees = getCachedActiveTees()
		if len(activeTees.TeeIds) == 0 {
			logger.Infof("No cached TEEs available, skipping this poll")
			return
		}
	} else {
		updateActiveTees(activeTees)
	}

	taskCh := make(chan task, len(activeTees.TeeIds))
	var wg sync.WaitGroup
	workers := min(defaultWorkerCount, len(activeTees.TeeIds))
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
					state, err := queryTeeInfoAndValidate(ctx, teeVerifier, t.proxyUrl)
					if err != nil {
						logger.Errorf("Failed to query teeInfo %s and validate: %v", t.proxyUrl, err)
					}
					teeVerifier.SamplesMu.Lock()
					samples := teeVerifier.TeeSamples[t.teeId]
					sample := teetypes.TeePollerSample{Timestamp: time.Now().UTC(), State: state}
					samples = append(samples, sample)
					if len(samples) > teeVerifier.SamplesToConsider {
						samples = samples[1:]
					}
					teeVerifier.TeeSamples[t.teeId] = samples
					teeVerifier.SamplesMu.Unlock()
				}
			}
		}()
	}
	for i, teeId := range activeTees.TeeIds {
		taskCh <- task{teeId: teeId, proxyUrl: activeTees.Urls[i]}
	}
	close(taskCh)
	wg.Wait()
	teeVerifier.SamplesMu.RLock()
	logger.Debugf("TEE poller samples: %v", teeVerifier.TeeSamples)
	teeVerifier.SamplesMu.RUnlock()
}

func queryTeeInfoAndValidate(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyUrl string) (teetypes.TeePollerSampleState, error) {
	infoResponse, err := fetchTEEInfoData(ctx, proxyUrl)
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
	state, err := teeVerifier.CheckSigningPolicies(infoData)
	if err != nil {
		return state, err
	}
	return state, nil
}

type teeList struct {
	TeeIds []common.Address
	Urls   []string
}

func getAllActiveTeeMachines(teeVerifier *verifier.TeeVerifier) (teeList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	callOpts := &bind.CallOpts{
		Context: ctx,
	}
	activeTees, err := teeVerifier.TeeMachineRegistryCaller.GetAllActiveTeeMachines(callOpts)
	if err != nil {
		return teeList{}, fmt.Errorf("getAllActiveTeeMachines: %w", err)
	}
	logger.Debugf("TEE poller got active Tees: %v", activeTees)
	return activeTees, nil
}

func getAllActiveTeesWithRetry(teeVerifier *verifier.TeeVerifier) (teeList, error) {
	return utils.Retry(chainRetries, chainRetryDelay, func() (teeList, error) {
		return getAllActiveTeeMachines(teeVerifier)
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
