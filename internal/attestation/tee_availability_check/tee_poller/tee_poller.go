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
	cancel         context.CancelFunc
	verifier       *verifier.TeeVerifier
	lastActiveTees teeList
	teesMu         sync.RWMutex
}

func (s *TeePollerService) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
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

func NewTeePoller(teeVerifier *verifier.TeeVerifier) *TeePollerService {
	return &TeePollerService{
		verifier: teeVerifier,
	}
}

func (s *TeePollerService) StartTeePoller(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	s.cancel = cancel

	go func() {
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("TEE poller recovered from panic: %v", r)
			}
		}()

		logger.Info("TEE poller started")
		// Run once immediately, before ticker starts.
		s.sampleAllTees(ctx, queryTeeInfoAndValidate)
		ticker := time.NewTicker(verifier.SampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sampleAllTees(ctx, queryTeeInfoAndValidate)
			case <-ctx.Done():
				logger.Infof("TEE poller stopped (%v)", ctx.Err())
				return
			}
		}
	}()
}

func (s *TeePollerService) sampleAllTees(
	ctx context.Context,
	queryInfoAndValidate func(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error),
) {
	activeTees, err := s.buildPollingList(ctx)
	if err != nil {
		logger.Warnf("Active TEE fetch failed; using cached list: %v", err)
		activeTees = s.getCachedActiveTees()
		if len(activeTees.TeeIDs) == 0 {
			logger.Infof("No cached TEEs; skipping poll")
			return
		}
	} else {
		if !s.activeTeesEqual(activeTees) {
			teeEntries := make([]string, len(activeTees.TeeIDs))
			for i, id := range activeTees.TeeIDs {
				teeEntries[i] = fmt.Sprintf("%s (%s)", id.Hex(), activeTees.URLs[i])
			}
			logger.Infof("TEE poller active TEEs changed: count=%d, TEEs=%v", len(activeTees.TeeIDs), teeEntries)
		}
		s.updateActiveTees(activeTees)
		s.filterTeeSamplesToActive(activeTees)
	}

	taskCh := make(chan task, len(activeTees.TeeIDs))
	var wg sync.WaitGroup
	workers := min(defaultWorkerCount, len(activeTees.TeeIDs))
	for range workers {
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("Worker recovered from panic: %v", r)
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
					state, validationErr := queryInfoAndValidate(ctx, s.verifier, proxyURL, t.teeID)

					// Update samples under lock, capture flags for logging outside lock.
					var isFirst, stateChanged bool
					var prevState verifiertypes.TeeSampleState
					s.verifier.SamplesMu.Lock()
					samples := s.verifier.TeeSamples[t.teeID]
					isFirst = len(samples) == 0
					if !isFirst {
						prevState = samples[len(samples)-1].State
						stateChanged = prevState != state
					}
					sample := verifiertypes.TeeSampleValue{Timestamp: time.Now().UTC(), State: state}
					samples = append(samples, sample)
					if len(samples) > verifier.SamplesToConsider {
						samples = samples[1:]
					}
					s.verifier.TeeSamples[t.teeID] = samples
					s.verifier.SamplesMu.Unlock()

					// Log outside lock to avoid blocking workers on I/O.
					if isFirst {
						logger.Debugf("TEE %s first sample: %v", t.teeID.Hex(), state)
						if validationErr != nil {
							logger.Warnf("TEE %s validation failed: %v", t.teeID.Hex(), validationErr)
						}
					} else if stateChanged {
						logger.Infof("TEE %s state changed: %v → %v", t.teeID.Hex(), prevState, state)
						if validationErr != nil {
							logger.Warnf("TEE %s validation failed: %v", t.teeID.Hex(), validationErr)
						}
					}
				}
			}
		})
	}
	for i, teeID := range activeTees.TeeIDs {
		taskCh <- task{teeID: teeID, proxyURL: activeTees.URLs[i]}
	}
	close(taskCh)
	wg.Wait()
	s.verifier.SamplesMu.RLock()
	var validCount, invalidCount, indeterminateCount int
	for _, samples := range s.verifier.TeeSamples {
		if len(samples) > 0 {
			switch samples[len(samples)-1].State {
			case verifiertypes.TeeSampleValid:
				validCount++
			case verifiertypes.TeeSampleInvalid:
				invalidCount++
			case verifiertypes.TeeSampleIndeterminate:
				indeterminateCount++
			}
		}
	}
	s.verifier.SamplesMu.RUnlock()
	logger.Infof("TEE poller cycle complete: total=%d, valid=%d, invalid=%d, indeterminate=%d",
		len(activeTees.TeeIDs), validCount, invalidCount, indeterminateCount)
}

func queryTeeInfoAndValidate(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
	infoResponse, err := fetchTEEInfoData(ctx, teeVerifier, proxyURL)
	if err != nil {
		return verifiertypes.TeeSampleInvalid, fmt.Errorf("cannot fetch TEE info from %s: %w", proxyURL, err)
	}
	checkInfoChallenge, err := teeVerifier.CheckInfoChallengeIsValid(ctx, infoResponse.TeeInfo.Challenge)
	if err != nil {
		return checkInfoChallenge, err
	}
	_, err = teeVerifier.DataVerification(ctx, infoResponse, teeID, true)
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

// buildPollingList fetches extension 0 TEEs (always polled) and optionally
// fills up to MaxPolledTees with remaining TEEs from other extensions.
func (s *TeePollerService) buildPollingList(ctx context.Context) (teeList, error) {
	// Always fetch extension 0 TEEs.
	ext0Tees, err := s.getExtensionTeesWithRetry(ctx, 0)
	if err != nil {
		return teeList{}, fmt.Errorf("fetching extension 0 TEEs: %w", err)
	}

	maxTees := s.verifier.Cfg.MaxPolledTees
	if maxTees <= 0 || maxTees <= len(ext0Tees.TeeIDs) {
		// Cap disabled (0) or extension 0 already meets/exceeds cap — poll only extension 0.
		return ext0Tees, nil
	}

	// Fetch all active TEEs and add non-extension-0 ones up to the cap.
	allTees, err := s.getAllActiveTeesWithRetry(ctx)
	if err != nil {
		// Fall back to extension 0 only.
		logger.Warnf("Failed to fetch all active TEEs, polling extension 0 only: %v", err)
		return ext0Tees, nil
	}

	// Build a set of extension 0 IDs for fast lookup.
	ext0Set := make(map[common.Address]struct{}, len(ext0Tees.TeeIDs))
	for _, id := range ext0Tees.TeeIDs {
		ext0Set[id] = struct{}{}
	}

	// Start with extension 0 TEEs, then add others up to the cap.
	result := teeList{
		TeeIDs: append([]common.Address{}, ext0Tees.TeeIDs...),
		URLs:   append([]string{}, ext0Tees.URLs...),
	}
	remaining := maxTees - len(result.TeeIDs)
	for i, id := range allTees.TeeIDs {
		if remaining <= 0 {
			break
		}
		if _, isExt0 := ext0Set[id]; !isExt0 {
			result.TeeIDs = append(result.TeeIDs, id)
			result.URLs = append(result.URLs, allTees.URLs[i])
			remaining--
		}
	}

	return result, nil
}

func (s *TeePollerService) getExtensionTees(ctx context.Context, extensionID int64) (teeList, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	callOpts := &bind.CallOpts{Context: ctx}

	tees, err := s.verifier.TeeMachineRegistryCaller.GetActiveTeeMachines(callOpts, big.NewInt(extensionID))
	if err != nil {
		return teeList{}, fmt.Errorf("getActiveTeeMachines(extensionId=%d) failed: %w", extensionID, err)
	}
	return teeList{
		TeeIDs: tees.TeeIds,
		URLs:   tees.Urls,
	}, nil
}

func (s *TeePollerService) getExtensionTeesWithRetry(ctx context.Context, extensionID int64) (teeList, error) {
	return fetcher.Retry(ctx, chainMaxAttempts, chainRetryDelay, func() (teeList, error) {
		return s.getExtensionTees(ctx, extensionID)
	}, nil)
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

	return activeTees, nil
}

func (s *TeePollerService) getAllActiveTeesWithRetry(ctx context.Context) (teeList, error) {
	return fetcher.Retry(ctx, chainMaxAttempts, chainRetryDelay, func() (teeList, error) {
		return s.getAllActiveTeeMachines(ctx, teeMachineChunk)
	}, nil)
}

func fetchTEEInfoData(ctx context.Context, teeVerifier *verifier.TeeVerifier, baseURL string) (teenodetype.TeeInfoResponse, error) {
	url := baseURL + "/info"
	resolved, err := verifier.ResolveExternalURL(ctx, baseURL, teeVerifier.Cfg.AllowPrivateNetworks)
	if err != nil {
		return teenodetype.TeeInfoResponse{}, err
	}
	dialAddr, hostHeader, serverName := verifier.BuildPinnedAddr(resolved)
	return fetcher.GetJSONPinned[teenodetype.TeeInfoResponse](ctx, url, fetchTimeout, dialAddr, hostHeader, serverName)
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

func (s *TeePollerService) activeTeesEqual(newTees teeList) bool {
	s.teesMu.RLock()
	defer s.teesMu.RUnlock()
	if len(s.lastActiveTees.TeeIDs) != len(newTees.TeeIDs) {
		return false
	}
	for i, id := range s.lastActiveTees.TeeIDs {
		if id != newTees.TeeIDs[i] || s.lastActiveTees.URLs[i] != newTees.URLs[i] {
			return false
		}
	}
	return true
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
			s.verifier.ClearMagicPassLogged(teeID)
			removedCount++
		}
	}
	if removedCount > 0 {
		logger.Debugf("Removed %d inactive TEE samples from cache", removedCount)
	}
}
