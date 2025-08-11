package poller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
)

const (
	SampleInterval     = 1 * time.Minute
	DefaultWorkerCount = 10
)

type task struct {
	teeId    common.Address
	proxyUrl string
}

func StartPoller(ctx context.Context, teeVerifier *verifier.TeeVerifier) {
	teeVerifier.TeeSamples = make(map[common.Address][]bool)
	go func() {
		ticker := time.NewTicker(SampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sampleAllTees(ctx, teeVerifier)
			case <-ctx.Done():
				logger.Infof("Poller stopped: %v", ctx.Err())
				return
			}
		}
	}()
}

func sampleAllTees(ctx context.Context, teeVerifier *verifier.TeeVerifier) {
	activeTees, err := getActiveTees(teeVerifier)
	if err != nil {
		logger.Errorf("Failed to: %v", err)
		return
	}
	taskCh := make(chan task, len(activeTees.TeeIds))
	var wg sync.WaitGroup
	workers := min(DefaultWorkerCount, len(activeTees.TeeIds))
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case t, ok := <-taskCh:
					if !ok {
						return
					}
					valid, err := queryTeeInfoAndValidate(ctx, teeVerifier, t.proxyUrl)
					if err != nil {
						logger.Errorf("Failed query teeInfo %s and validate: %v", t.proxyUrl, err)
					}
					teeVerifier.SamplesMu.Lock()
					samples := teeVerifier.TeeSamples[t.teeId]
					samples = append(samples, valid)
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
}

func queryTeeInfoAndValidate(ctx context.Context, teeVerifier *verifier.TeeVerifier, proxyUrl string) (bool, error) {
	valid, err := teeVerifier.FetchTEEInfoResultAndValidate(ctx, proxyUrl)
	if err != nil {
		return false, err
	}
	return valid, nil
}

type teeList struct {
	TeeIds []common.Address
	Urls   []string
}

func getActiveTees(teeVerifier *verifier.TeeVerifier) (teeList, error) {
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}

	activeTees, err := teeVerifier.TeeMachineRegistryCaller.GetAllActiveTeeMachines(callOpts)
	if err != nil {
		return teeList{}, fmt.Errorf("getActiveTees: %w", err)
	}
	return activeTees, nil
}
