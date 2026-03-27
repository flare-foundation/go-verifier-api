//go:build load

package teepoller

import (
	"context"
	"fmt"
	"math/big"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// mockValidateFunc returns a queryInfoAndValidate function that simulates
// a mix of healthy and slow/unhealthy TEEs.
func mockValidateFunc(slowIDs map[common.Address]time.Duration) func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
	return func(ctx context.Context, v *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
		if delay, ok := slowIDs[teeID]; ok {
			select {
			case <-time.After(delay):
				return verifiertypes.TeeSampleInvalid, fmt.Errorf("slow TEE %s timed out", teeID.Hex())
			case <-ctx.Done():
				return verifiertypes.TeeSampleIndeterminate, ctx.Err()
			}
		}
		select {
		case <-time.After(10 * time.Millisecond):
			return verifiertypes.TeeSampleValid, nil
		case <-ctx.Done():
			return verifiertypes.TeeSampleIndeterminate, ctx.Err()
		}
	}
}

func makeTeeList(count int) (teeList, []common.Address) {
	ids := make([]common.Address, count)
	urls := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = common.BigToAddress(big.NewInt(int64(i + 1)))
		urls[i] = fmt.Sprintf("http://tee-%d.example.com", i)
	}
	return teeList{TeeIDs: ids, URLs: urls}, ids
}

// TestLoadPollerSingleCycleCompletion verifies that a single poll cycle
// completes within a reasonable time for varying numbers of TEEs.
func TestLoadPollerSingleCycleCompletion(t *testing.T) {
	for _, teeCount := range []int{10, 50, 100, 200, 500} {
		t.Run(fmt.Sprintf("%d_TEEs", teeCount), func(t *testing.T) {
			activeTees, _ := makeTeeList(teeCount)

			v := &verifier.TeeVerifier{
				Cfg:        &config.TeeAvailabilityCheckConfig{AllowPrivateNetworks: true},
				TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
				TeeMachineRegistryCaller: &mockTeeMachineRegistryCaller{
					getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
						s := int(start.Int64())
						e := int(end.Int64())
						if s >= len(activeTees.TeeIDs) {
							return teeMachinesResult{TotalLength: big.NewInt(int64(len(activeTees.TeeIDs)))}, nil
						}
						if e > len(activeTees.TeeIDs) {
							e = len(activeTees.TeeIDs)
						}
						return teeMachinesResult{
							TeeIds:      activeTees.TeeIDs[s:e],
							Urls:        activeTees.URLs[s:e],
							TotalLength: big.NewInt(int64(len(activeTees.TeeIDs))),
						}, nil
					},
				},
			}

			poller := NewTeePoller(v)
			validate := mockValidateFunc(nil) // all healthy

			start := time.Now()
			poller.sampleAllTees(context.Background(), validate)
			elapsed := time.Since(start)

			// Cycle must complete well within the 1-minute polling interval.
			// With 10 workers and 10ms per TEE, expect ~teeCount/10 * 10ms + overhead.
			maxExpected := time.Duration(teeCount/defaultWorkerCount+1) * 50 * time.Millisecond
			if maxExpected < time.Second {
				maxExpected = time.Second
			}
			if elapsed > maxExpected {
				t.Fatalf("%d TEEs: cycle took %v, expected under %v", teeCount, elapsed, maxExpected)
			}

			// All TEEs should have exactly 1 sample.
			v.SamplesMu.RLock()
			sampleCount := len(v.TeeSamples)
			v.SamplesMu.RUnlock()
			if sampleCount != teeCount {
				t.Fatalf("expected %d TEEs in sample map, got %d", teeCount, sampleCount)
			}

			t.Logf("%d TEEs: cycle completed in %v", teeCount, elapsed)
		})
	}
}

// TestLoadPollerSlowTEEsDoNotStallCycle verifies that slow/unhealthy TEEs
// don't disproportionately stall the cycle for healthy TEEs.
func TestLoadPollerSlowTEEsDoNotStallCycle(t *testing.T) {
	activeTees, ids := makeTeeList(100)

	// Make 25 out of 100 TEEs slow (2 seconds each).
	slowIDs := make(map[common.Address]time.Duration)
	for i := 0; i < 25; i++ {
		slowIDs[ids[i]] = 2 * time.Second
	}

	v := &verifier.TeeVerifier{
		Cfg:        &config.TeeAvailabilityCheckConfig{AllowPrivateNetworks: true},
		TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
		TeeMachineRegistryCaller: &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				s := int(start.Int64())
				e := int(end.Int64())
				if s >= len(activeTees.TeeIDs) {
					return teeMachinesResult{TotalLength: big.NewInt(int64(len(activeTees.TeeIDs)))}, nil
				}
				if e > len(activeTees.TeeIDs) {
					e = len(activeTees.TeeIDs)
				}
				return teeMachinesResult{
					TeeIds:      activeTees.TeeIDs[s:e],
					Urls:        activeTees.URLs[s:e],
					TotalLength: big.NewInt(int64(len(activeTees.TeeIDs))),
				}, nil
			},
		},
	}

	poller := NewTeePoller(v)
	validate := mockValidateFunc(slowIDs)

	start := time.Now()
	poller.sampleAllTees(context.Background(), validate)
	elapsed := time.Since(start)

	// With 10 workers and 25 slow TEEs (2s each), the cycle should complete
	// in roughly 6s (3 batches of ~8-9 slow TEEs at 2s), not 50s (if sequential).
	maxExpected := 10 * time.Second
	if elapsed > maxExpected {
		t.Fatalf("cycle took %v, expected under %v — slow TEEs may be stalling the pool", elapsed, maxExpected)
	}

	// Count valid vs invalid samples.
	v.SamplesMu.RLock()
	validCount := 0
	invalidCount := 0
	for _, samples := range v.TeeSamples {
		for _, s := range samples {
			if s.State == verifiertypes.TeeSampleValid {
				validCount++
			} else {
				invalidCount++
			}
		}
	}
	v.SamplesMu.RUnlock()

	if validCount != 75 {
		t.Fatalf("expected 75 valid samples, got %d", validCount)
	}
	if invalidCount != 25 {
		t.Fatalf("expected 25 invalid samples, got %d", invalidCount)
	}

	t.Logf("100 TEEs (25 slow): cycle=%v, valid=%d, invalid=%d", elapsed, validCount, invalidCount)
}

// TestLoadPollerRepeatedCyclesStability runs multiple poll cycles and verifies
// goroutine count and sample-map size stay stable.
func TestLoadPollerRepeatedCyclesStability(t *testing.T) {
	const (
		teeCount = 100
		cycles   = 20
	)

	activeTees, _ := makeTeeList(teeCount)

	var validateCalls int64
	v := &verifier.TeeVerifier{
		Cfg:        &config.TeeAvailabilityCheckConfig{AllowPrivateNetworks: true},
		TeeSamples: make(map[common.Address][]verifiertypes.TeeSampleValue),
		TeeMachineRegistryCaller: &mockTeeMachineRegistryCaller{
			getAllActiveFunc: func(opts *bind.CallOpts, start, end *big.Int) (teeMachinesResult, error) {
				s := int(start.Int64())
				e := int(end.Int64())
				if s >= len(activeTees.TeeIDs) {
					return teeMachinesResult{TotalLength: big.NewInt(int64(len(activeTees.TeeIDs)))}, nil
				}
				if e > len(activeTees.TeeIDs) {
					e = len(activeTees.TeeIDs)
				}
				return teeMachinesResult{
					TeeIds:      activeTees.TeeIDs[s:e],
					Urls:        activeTees.URLs[s:e],
					TotalLength: big.NewInt(int64(len(activeTees.TeeIDs))),
				}, nil
			},
		},
	}

	poller := NewTeePoller(v)
	validate := func(ctx context.Context, ver *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
		atomic.AddInt64(&validateCalls, 1)
		select {
		case <-time.After(5 * time.Millisecond):
			return verifiertypes.TeeSampleValid, nil
		case <-ctx.Done():
			return verifiertypes.TeeSampleIndeterminate, ctx.Err()
		}
	}

	baseGoroutines := runtime.NumGoroutine()
	cycleDurations := make([]time.Duration, cycles)

	for cycle := 0; cycle < cycles; cycle++ {
		start := time.Now()
		poller.sampleAllTees(context.Background(), validate)
		cycleDurations[cycle] = time.Since(start)
	}

	// Goroutine count should not grow — all workers from each cycle should be cleaned up.
	finalGoroutines := runtime.NumGoroutine()
	goroutineGrowth := finalGoroutines - baseGoroutines
	if goroutineGrowth > 5 { // small tolerance for runtime/test goroutines
		t.Fatalf("goroutine leak: started with %d, ended with %d (growth: %d)", baseGoroutines, finalGoroutines, goroutineGrowth)
	}

	// Sample map should have exactly teeCount entries, each with SamplesToConsider samples.
	v.SamplesMu.RLock()
	mapSize := len(v.TeeSamples)
	var maxSamples int
	for _, samples := range v.TeeSamples {
		if len(samples) > maxSamples {
			maxSamples = len(samples)
		}
	}
	v.SamplesMu.RUnlock()

	if mapSize != teeCount {
		t.Fatalf("expected %d TEEs in sample map, got %d", teeCount, mapSize)
	}
	// After 20 cycles, samples should be capped at SamplesToConsider (5).
	if maxSamples != verifier.SamplesToConsider {
		t.Fatalf("expected max %d samples per TEE, got %d", verifier.SamplesToConsider, maxSamples)
	}

	totalCalls := atomic.LoadInt64(&validateCalls)
	expectedCalls := int64(teeCount * cycles)
	if totalCalls != expectedCalls {
		t.Fatalf("expected %d validate calls, got %d", expectedCalls, totalCalls)
	}

	var minCycle, maxCycle time.Duration
	minCycle = cycleDurations[0]
	for _, d := range cycleDurations {
		if d < minCycle {
			minCycle = d
		}
		if d > maxCycle {
			maxCycle = d
		}
	}

	t.Logf("Stability: %d cycles × %d TEEs, goroutine growth=%d, max samples/TEE=%d, total validate calls=%d, cycle min=%v max=%v",
		cycles, teeCount, goroutineGrowth, maxSamples, totalCalls, minCycle, maxCycle)
}
