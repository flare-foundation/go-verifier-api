//go:build stress

package verifier

import (
	"context"
	"crypto/x509"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStressCRLCacheConcurrencyRamp ramps concurrency from 100 to 1000
// with many unique URLs to defeat caching. Measures where latency degrades.
func TestStressCRLCacheConcurrencyRamp(t *testing.T) {
	caCert, caKey := generateTestCert(t, true, nil, nil, nil)
	crlBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

	for _, concurrency := range []int{100, 250, 500, 1000} {
		t.Run(fmt.Sprintf("%d_concurrent", concurrency), func(t *testing.T) {
			var fetchCount int64
			cache := &CRLCache{
				entries: make(map[string]*crlEntry),
				fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
					atomic.AddInt64(&fetchCount, 1)
					time.Sleep(10 * time.Millisecond)
					return crlBytes, nil
				},
			}

			// Each goroutine uses a unique URL to defeat caching.
			type callResult struct {
				crl     *x509.RevocationList
				err     error
				elapsed time.Duration
			}
			results := make([]callResult, concurrency)
			var wg sync.WaitGroup
			wg.Add(concurrency)

			start := time.Now()
			for i := 0; i < concurrency; i++ {
				go func(idx int) {
					defer wg.Done()
					url := fmt.Sprintf("http://example.com/crl-%d", idx)
					s := time.Now()
					crl, err := cache.getOrFetchCRL(context.Background(), url, caCert)
					results[idx] = callResult{crl: crl, err: err, elapsed: time.Since(s)}
				}(i)
			}
			wg.Wait()
			totalElapsed := time.Since(start)

			var errors int
			latencies := make([]time.Duration, 0, concurrency)
			for _, r := range results {
				if r.err != nil {
					errors++
				}
				latencies = append(latencies, r.elapsed)
			}

			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			n := len(latencies)
			t.Logf("concurrency=%d: total=%v, p50=%v, p95=%v, p99=%v, errors=%d, fetches=%d, cache_size=%d",
				concurrency, totalElapsed,
				latencies[n*50/100], latencies[n*95/100], latencies[n*99/100],
				errors, atomic.LoadInt64(&fetchCount), len(cache.entries))

			if errors > 0 {
				t.Errorf("unexpected errors at concurrency %d: %d", concurrency, errors)
			}
		})
	}
}

// TestStressCRLCacheSlowUpstream verifies that slow/failing CRL endpoints
// don't cause goroutine pileup or collateral slowdown for healthy URLs.
func TestStressCRLCacheSlowUpstream(t *testing.T) {
	caCert, caKey := generateTestCert(t, true, nil, nil, nil)
	crlBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

	cache := &CRLCache{
		entries: make(map[string]*crlEntry),
		fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
			if strings.HasPrefix(url, "http://slow.example.com/") {
				select {
				case <-time.After(5 * time.Second):
					return nil, fmt.Errorf("slow upstream timeout")
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			time.Sleep(5 * time.Millisecond)
			return crlBytes, nil
		},
	}

	const (
		healthyCount = 100
		slowCount    = 50
	)

	type callResult struct {
		url     string
		err     error
		elapsed time.Duration
	}

	results := make([]callResult, healthyCount+slowCount)
	var wg sync.WaitGroup
	wg.Add(healthyCount + slowCount)

	baseGoroutines := runtime.NumGoroutine()

	// Launch healthy requests.
	for i := 0; i < healthyCount; i++ {
		go func(idx int) {
			defer wg.Done()
			url := fmt.Sprintf("http://fast.example.com/crl-%d", idx)
			s := time.Now()
			_, err := cache.getOrFetchCRL(context.Background(), url, caCert)
			results[idx] = callResult{url: url, err: err, elapsed: time.Since(s)}
		}(i)
	}

	// Launch slow requests — each with a distinct URL to avoid singleflight dedup.
	for i := 0; i < slowCount; i++ {
		go func(idx int) {
			defer wg.Done()
			url := fmt.Sprintf("http://slow.example.com/crl-%d", idx)
			s := time.Now()
			_, err := cache.getOrFetchCRL(context.Background(), url, caCert)
			results[healthyCount+idx] = callResult{url: url, err: err, elapsed: time.Since(s)}
		}(i)
	}

	wg.Wait()

	// Analyze healthy vs slow results.
	var healthyLatencies []time.Duration
	var healthyErrors, slowErrors int
	for i := 0; i < healthyCount; i++ {
		if results[i].err != nil {
			healthyErrors++
		}
		healthyLatencies = append(healthyLatencies, results[i].elapsed)
	}
	for i := healthyCount; i < healthyCount+slowCount; i++ {
		if results[i].err != nil {
			slowErrors++
		}
	}

	sort.Slice(healthyLatencies, func(i, j int) bool { return healthyLatencies[i] < healthyLatencies[j] })
	n := len(healthyLatencies)

	finalGoroutines := runtime.NumGoroutine()

	t.Logf("Healthy (n=%d): p50=%v, p95=%v, p99=%v, errors=%d",
		n, healthyLatencies[n*50/100], healthyLatencies[n*95/100], healthyLatencies[n*99/100], healthyErrors)
	t.Logf("Slow (n=%d): errors=%d (expected — slow upstream)", slowCount, slowErrors)
	t.Logf("Goroutines: before=%d, after=%d", baseGoroutines, finalGoroutines)

	// Healthy requests should not be blocked by slow ones.
	if healthyLatencies[n*95/100] > 500*time.Millisecond {
		t.Errorf("healthy p95 (%v) too high — slow upstream may be causing collateral slowdown", healthyLatencies[n*95/100])
	}
	if healthyErrors > 0 {
		t.Errorf("healthy requests had %d errors — slow upstream should not affect them", healthyErrors)
	}
}

// TestStressCRLCacheSustained runs CRL cache operations for a sustained period
// and checks for memory/goroutine growth.
func TestStressCRLCacheSustained(t *testing.T) {
	caCert, caKey := generateTestCert(t, true, nil, nil, nil)
	crlBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

	cache := &CRLCache{
		entries: make(map[string]*crlEntry),
		fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
			time.Sleep(2 * time.Millisecond)
			return crlBytes, nil
		},
	}

	const (
		duration    = 30 * time.Second
		concurrency = 50
		urlCount    = 200 // rotate through URLs to test cache eviction
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var totalOps int64
	var totalErrors int64
	baseGoroutines := runtime.NumGoroutine()
	var baseHeap uint64
	{
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		baseHeap = m.HeapInuse
	}

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				url := fmt.Sprintf("http://example.com/crl-%d", counter%urlCount)
				counter++
				_, err := cache.getOrFetchCRL(ctx, url, caCert)
				atomic.AddInt64(&totalOps, 1)
				if err != nil && ctx.Err() == nil {
					atomic.AddInt64(&totalErrors, 1)
				}
			}
		}(i)
	}
	wg.Wait()

	finalGoroutines := runtime.NumGoroutine()
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	ops := atomic.LoadInt64(&totalOps)
	errs := atomic.LoadInt64(&totalErrors)
	goroutineGrowth := finalGoroutines - baseGoroutines

	// Use HeapInuse for stable comparison (HeapAlloc fluctuates with GC).
	heapGrowthMB := float64(int64(finalMem.HeapInuse)-int64(baseHeap)) / 1024 / 1024
	if heapGrowthMB < 0 {
		heapGrowthMB = 0
	}

	t.Logf("Sustained %v: ops=%d (%.0f/sec), errors=%d, heap_growth=%.1fMB, goroutine_growth=%d, cache_size=%d",
		duration, ops, float64(ops)/duration.Seconds(), errs,
		heapGrowthMB, goroutineGrowth, len(cache.entries))

	if errs > 0 {
		t.Errorf("unexpected errors during sustained run: %d", errs)
	}
	if goroutineGrowth > 10 {
		t.Errorf("goroutine leak: growth=%d", goroutineGrowth)
	}
	if heapGrowthMB > 50 {
		t.Errorf("excessive heap growth: %.1fMB", heapGrowthMB)
	}
}
