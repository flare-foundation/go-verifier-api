//go:build stress

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

func stressMakeTeeList(count int) teeList {
	ids := make([]common.Address, count)
	urls := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = common.BigToAddress(big.NewInt(int64(i + 1)))
		urls[i] = fmt.Sprintf("http://tee-%d.example.com", i)
	}
	return teeList{TeeIDs: ids, URLs: urls}
}

func stressMockRegistry(activeTees teeList) *mockTeeMachineRegistryCaller {
	return &mockTeeMachineRegistryCaller{
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
	}
}

// TestStressPollerConcurrencyRamp increases TEE count to find where
// cycle time exceeds the 1-minute polling interval.
func TestStressPollerConcurrencyRamp(t *testing.T) {
	for _, teeCount := range []int{100, 250, 500, 1000} {
		t.Run(fmt.Sprintf("%d_TEEs", teeCount), func(t *testing.T) {
			activeTees := stressMakeTeeList(teeCount)
			v := &verifier.TeeVerifier{
				Cfg:                      &config.TeeAvailabilityCheckConfig{AllowPrivateNetworks: true},
				TeeSamples:               make(map[common.Address][]verifiertypes.TeeSampleValue),
				TeeMachineRegistryCaller: stressMockRegistry(activeTees),
			}

			validate := func(ctx context.Context, ver *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
				select {
				case <-time.After(10 * time.Millisecond):
					return verifiertypes.TeeSampleValid, nil
				case <-ctx.Done():
					return verifiertypes.TeeSampleIndeterminate, ctx.Err()
				}
			}

			poller := NewTeePoller(v)
			start := time.Now()
			poller.sampleAllTees(context.Background(), validate)
			elapsed := time.Since(start)

			exceeds := elapsed > time.Minute
			marker := ""
			if exceeds {
				marker = " ** EXCEEDS 1-MINUTE INTERVAL **"
			}

			v.SamplesMu.RLock()
			sampleCount := len(v.TeeSamples)
			v.SamplesMu.RUnlock()

			t.Logf("%d TEEs: cycle=%v, samples=%d%s", teeCount, elapsed, sampleCount, marker)

			if sampleCount != teeCount {
				t.Errorf("expected %d samples, got %d", teeCount, sampleCount)
			}
		})
	}
}

// TestStressPollerSlowUpstreamIsolation verifies that a large proportion
// of slow TEEs doesn't prevent healthy TEEs from being sampled promptly.
func TestStressPollerSlowUpstreamIsolation(t *testing.T) {
	const totalTEEs = 200
	activeTees := stressMakeTeeList(totalTEEs)

	// First 100 TEEs are slow (3s each), rest are fast.
	slowSet := make(map[common.Address]bool)
	for i := 0; i < 100; i++ {
		slowSet[activeTees.TeeIDs[i]] = true
	}

	v := &verifier.TeeVerifier{
		Cfg:                      &config.TeeAvailabilityCheckConfig{AllowPrivateNetworks: true},
		TeeSamples:               make(map[common.Address][]verifiertypes.TeeSampleValue),
		TeeMachineRegistryCaller: stressMockRegistry(activeTees),
	}

	validate := func(ctx context.Context, ver *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
		if slowSet[teeID] {
			select {
			case <-time.After(3 * time.Second):
				return verifiertypes.TeeSampleInvalid, fmt.Errorf("slow TEE")
			case <-ctx.Done():
				return verifiertypes.TeeSampleIndeterminate, ctx.Err()
			}
		}
		select {
		case <-time.After(5 * time.Millisecond):
			return verifiertypes.TeeSampleValid, nil
		case <-ctx.Done():
			return verifiertypes.TeeSampleIndeterminate, ctx.Err()
		}
	}

	poller := NewTeePoller(v)
	start := time.Now()
	poller.sampleAllTees(context.Background(), validate)
	elapsed := time.Since(start)

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

	t.Logf("200 TEEs (100 slow @ 3s): cycle=%v, valid=%d, invalid=%d", elapsed, validCount, invalidCount)

	if validCount != 100 {
		t.Errorf("expected 100 valid, got %d", validCount)
	}
	if invalidCount != 100 {
		t.Errorf("expected 100 invalid, got %d", invalidCount)
	}
}

// TestStressPollerSustained runs multiple poll cycles over a sustained period
// and monitors goroutine/memory stability.
func TestStressPollerSustained(t *testing.T) {
	const (
		teeCount = 100
		cycles   = 50
	)

	activeTees := stressMakeTeeList(teeCount)
	var validateCalls int64

	v := &verifier.TeeVerifier{
		Cfg:                      &config.TeeAvailabilityCheckConfig{AllowPrivateNetworks: true},
		TeeSamples:               make(map[common.Address][]verifiertypes.TeeSampleValue),
		TeeMachineRegistryCaller: stressMockRegistry(activeTees),
	}

	validate := func(ctx context.Context, ver *verifier.TeeVerifier, proxyURL string, teeID common.Address) (verifiertypes.TeeSampleState, error) {
		atomic.AddInt64(&validateCalls, 1)
		select {
		case <-time.After(5 * time.Millisecond):
			return verifiertypes.TeeSampleValid, nil
		case <-ctx.Done():
			return verifiertypes.TeeSampleIndeterminate, ctx.Err()
		}
	}

	poller := NewTeePoller(v)

	baseGoroutines := runtime.NumGoroutine()
	var baseHeap uint64
	{
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		baseHeap = m.HeapInuse
	}

	cycleDurations := make([]time.Duration, cycles)
	for i := 0; i < cycles; i++ {
		start := time.Now()
		poller.sampleAllTees(context.Background(), validate)
		cycleDurations[i] = time.Since(start)
	}

	finalGoroutines := runtime.NumGoroutine()
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	goroutineGrowth := finalGoroutines - baseGoroutines
	heapGrowthMB := float64(int64(finalMem.HeapInuse)-int64(baseHeap)) / 1024 / 1024
	if heapGrowthMB < 0 {
		heapGrowthMB = 0
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

	v.SamplesMu.RLock()
	var maxSamples int
	for _, samples := range v.TeeSamples {
		if len(samples) > maxSamples {
			maxSamples = len(samples)
		}
	}
	v.SamplesMu.RUnlock()

	t.Logf("Sustained: %d cycles × %d TEEs, calls=%d, cycle min=%v max=%v, goroutine_growth=%d, heap_growth=%.1fMB, max_samples=%d",
		cycles, teeCount, atomic.LoadInt64(&validateCalls),
		minCycle, maxCycle, goroutineGrowth, heapGrowthMB, maxSamples)

	if goroutineGrowth > 10 {
		t.Errorf("goroutine leak: growth=%d", goroutineGrowth)
	}
	if heapGrowthMB > 50 {
		t.Errorf("excessive heap growth: %.1fMB", heapGrowthMB)
	}
	if maxSamples != verifier.SamplesToConsider {
		t.Errorf("sample cap violated: max=%d, expected=%d", maxSamples, verifier.SamplesToConsider)
	}
}
