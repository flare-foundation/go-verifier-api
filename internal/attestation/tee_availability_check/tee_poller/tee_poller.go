package teepoller

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/fetcher"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
	teenodetype "github.com/flare-foundation/tee-node/pkg/types"
)

const (
	defaultWorkerCount = 10
	fetchTimeout       = 5 * time.Second
	chainMaxAttempts   = 2
	chainRetryDelay    = 500 * time.Millisecond
	teeMachineChunk    = 100
)

type TeePollerService struct {
	ctx            context.Context
	cancel         context.CancelFunc
	verifier       *verifier.TeeVerifier
	lastActiveTees teeList
	teesMu         sync.RWMutex
}

func (s *TeePollerService) Close() error {
	s.cancel()
	return nil
}

// Ensure *TeePollerService implements io.Closer.
var _ io.Closer = (*TeePollerService)(nil)

type TeePollerSample struct {
	Timestamp time.Time
	State     verifiertypes.TeeSampleState
}

type task struct {
	teeID    common.Address
	proxyURL string
}

func NewTeePoller(parentCtx context.Context, teeVerifier *verifier.TeeVerifier) *TeePollerService {
	ctx, cancel := context.WithCancel(parentCtx)
	return &TeePollerService{
		ctx:      ctx,
		cancel:   cancel,
		verifier: teeVerifier,
	}
}

func (s *TeePollerService) StartTeePoller() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("TEE poller panic recovered: %v", r)
			}
		}()
		logger.Info("TEE poller started")
		// Run once immediately, before ticker starts.
		s.sampleAllTees(s.ctx, queryTeeInfoAndValidate)
		ticker := time.NewTicker(verifier.SampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sampleAllTees(s.ctx, queryTeeInfoAndValidate)
			case <-s.ctx.Done():
				logger.Infof("TEE poller stopped: %v", s.ctx.Err())
				return
			}
		}
	}()
}

func (s *TeePollerService) sampleAllTees(
	ctx context.Context,
	queryInfoAndValidate func(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error),
) {
	activeTees, err := s.getAllActiveTeesWithRetry(ctx)
	if err != nil {
		logger.Warnf("Failed to fetch active TEEs from TeeMachineRegistry, using last cached version: %v", err)
		activeTees = s.getCachedActiveTees()
		if len(activeTees.TeeIDs) == 0 {
			logger.Infof("No cached TEEs available, skipping this poll")
			return
		}
	} else {
		s.updateActiveTees(activeTees)
		s.filterTeeSamplesToActive(activeTees)
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
					logger.Errorf("Worker panic recovered: %v", r)
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
					proxyURL := s.verifier.FormatProxyURL(t.proxyURL)
					state, err := queryInfoAndValidate(ctx, s.verifier, proxyURL, t.teeID)
					if err != nil {
						logger.Errorf("Failed to validate %s: %v", t.teeID.Hex(), err)
					}
					logger.Debugf("TEE %s state updated to %v", t.teeID.Hex(), state)
					s.verifier.SamplesMu.Lock()
					samples := s.verifier.TeeSamples[t.teeID]
					sample := verifiertypes.TeeSampleValue{Timestamp: time.Now().UTC(), State: state}
					samples = append(samples, sample)
					if len(samples) > verifier.SamplesToConsider {
						samples = samples[1:]
					}
					s.verifier.TeeSamples[t.teeID] = samples
					s.verifier.SamplesMu.Unlock()
				}
			}
		}()
	}
	for i, teeID := range activeTees.TeeIDs {
		taskCh <- task{teeID: teeID, proxyURL: activeTees.URLs[i]}
	}
	close(taskCh)
	wg.Wait()
	s.verifier.SamplesMu.RLock()
	logger.Debugf("TEE poller samples snapshot: %v", s.verifier.TeeSamples)
	s.verifier.SamplesMu.RUnlock()
}

func queryTeeInfoAndValidate(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
	infoResponse, err := fetchTEEInfoData(ctx, proxyURL)
	if err != nil {
		return verifiertypes.TeeSampleInvalid, fmt.Errorf("cannot fetch TEE info from %s: %w", proxyURL, err)
	}
	checkInfoChallenge, err := teeVerifier.CheckInfoChallengeIsValid(ctx, infoResponse.TeeInfo.Challenge)
	if err != nil {
		return checkInfoChallenge, err
	}
	_, err = teeVerifier.DataVerification(infoResponse, teeID)
	if err != nil {
		return verifiertypes.TeeSampleInvalid, fmt.Errorf("data verification failed for TEE %s: %w", teeID.Hex(), err)
	}
	infoData := infoResponse.TeeInfo
	state, err := teeVerifier.CheckSigningPolicies(ctx, infoData)
	if err != nil {
		return state, fmt.Errorf("signing policy check failed for TEE %s: %w", teeID.Hex(), err)
	}
	return state, nil
}

type teeList struct {
	TeeIDs []common.Address
	URLs   []string
}

func (s *TeePollerService) getAllActiveTeeMachines(ctx context.Context, teeChunk int64) (teeList, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	callOpts := &bind.CallOpts{
		Context: ctx,
	}

	var allTeeIDs []common.Address
	var allURLs []string
	start := big.NewInt(0)
	chunk := big.NewInt(teeChunk)
	for {
		tees, err := s.verifier.TeeMachineRegistryCaller.GetAllActiveTeeMachines(callOpts, start, new(big.Int).Add(start, chunk))
		if err != nil {
			return teeList{}, fmt.Errorf("getAllActiveTeeMachines(start=%d, chunk=%d) failed: %w", start.Int64(), chunk.Int64(), err)
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

	logger.Debugf("TEE poller retrieved active TEEs: %v", activeTees)

	return activeTees, nil
}

func (s *TeePollerService) getAllActiveTeesWithRetry(ctx context.Context) (teeList, error) {
	return fetcher.Retry(chainMaxAttempts, chainRetryDelay, func() (teeList, error) {
		return s.getAllActiveTeeMachines(ctx, teeMachineChunk)
	}, nil)
}

func fetchTEEInfoData(ctx context.Context, baseURL string) (teenodetype.TeeInfoResponse, error) {
	url := fmt.Sprintf("%s/info", baseURL)
	return fetcher.GetJSON[teenodetype.TeeInfoResponse](ctx, url, fetchTimeout)
}

func (s *TeePollerService) updateActiveTees(teelist teeList) {
	s.teesMu.Lock()
	defer s.teesMu.Unlock()
	s.lastActiveTees = teelist
}

func (s *TeePollerService) getCachedActiveTees() teeList {
	s.teesMu.RLock()
	defer s.teesMu.RUnlock()
	return s.lastActiveTees
}

func (s *TeePollerService) filterTeeSamplesToActive(activeTees teeList) {
	activeSet := make(map[common.Address]struct{}, len(activeTees.TeeIDs))
	for _, id := range activeTees.TeeIDs {
		activeSet[id] = struct{}{}
	}

	s.verifier.SamplesMu.Lock()
	defer s.verifier.SamplesMu.Unlock()

	removedCount := 0
	for teeID := range s.verifier.TeeSamples {
		if _, ok := activeSet[teeID]; !ok {
			delete(s.verifier.TeeSamples, teeID)
			removedCount++
		}
	}
	if removedCount > 0 {
		logger.Debugf("Removed %d inactive TEE samples from cache", removedCount)
	}
}
