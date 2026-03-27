//go:build load

package verifier

import (
	"context"
	"crypto/x509"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// latencyStats collects and reports latency percentiles.
type latencyStats struct {
	mu      sync.Mutex
	samples []time.Duration
	fetches int64
	errors  int64
}

func (s *latencyStats) record(d time.Duration) {
	s.mu.Lock()
	s.samples = append(s.samples, d)
	s.mu.Unlock()
}

func (s *latencyStats) report(t *testing.T, label string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()

	n := len(s.samples)
	if n == 0 {
		t.Logf("%s: no samples", label)
		return
	}

	sort.Slice(s.samples, func(i, j int) bool { return s.samples[i] < s.samples[j] })
	p50 := s.samples[n*50/100]
	p95 := s.samples[n*95/100]
	p99 := s.samples[n*99/100]

	t.Logf("%s: n=%d, p50=%v, p95=%v, p99=%v, errors=%d, fetches=%d",
		label, n, p50, p95, p99, atomic.LoadInt64(&s.errors), atomic.LoadInt64(&s.fetches))
}

// TestLoadCRLCacheConcurrentBurst simulates the data provider burst pattern:
// 12 goroutines hitting the same CRL URLs simultaneously across 50 rounds.
// Verifies singleflight deduplication, result consistency, and latency.
func TestLoadCRLCacheConcurrentBurst(t *testing.T) {
	caCert, caKey := generateTestCert(t, true, nil, nil, nil)
	leafCRLBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

	var fetchCount int64
	cache := &CRLCache{
		entries: make(map[string]*crlEntry),
		fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
			atomic.AddInt64(&fetchCount, 1)
			time.Sleep(20 * time.Millisecond)
			return leafCRLBytes, nil
		},
	}

	const (
		burstSize = 100
		rounds    = 50
		numURLs   = 3
	)

	type callResult struct {
		crl *x509.RevocationList
		err error
	}

	stats := &latencyStats{}

	for round := 0; round < rounds; round++ {
		cache.mu.Lock()
		cache.entries = make(map[string]*crlEntry)
		cache.mu.Unlock()
		atomic.StoreInt64(&fetchCount, 0)

		var wg sync.WaitGroup
		results := make([]callResult, burstSize)

		wg.Add(burstSize)
		for i := 0; i < burstSize; i++ {
			go func(idx int) {
				defer wg.Done()
				url := fmt.Sprintf("http://example.com/crl-%d", idx%numURLs)
				start := time.Now()
				crl, err := cache.getOrFetchCRL(context.Background(), url, caCert)
				stats.record(time.Since(start))
				results[idx] = callResult{crl: crl, err: err}
				if err != nil {
					atomic.AddInt64(&stats.errors, 1)
				}
			}(i)
		}
		wg.Wait()

		// Assert from parent goroutine after wg.Wait.
		for i, r := range results {
			if r.err != nil {
				t.Fatalf("round %d, caller %d: unexpected error: %v", round, i, r.err)
			}
			if r.crl == nil {
				t.Fatalf("round %d, caller %d: got nil CRL", round, i)
			}
		}

		// All callers for the same URL got the same CRL pointer (shared via singleflight).
		for i := numURLs; i < burstSize; i++ {
			if results[i].crl != results[i%numURLs].crl {
				t.Fatalf("round %d: callers for URL index %d got different CRL pointers", round, i%numURLs)
			}
		}

		// Singleflight deduplicates: at most numURLs fetches per round.
		fetches := atomic.LoadInt64(&fetchCount)
		if fetches > int64(numURLs) {
			t.Fatalf("round %d: expected at most %d fetches, got %d", round, numURLs, fetches)
		}
		atomic.AddInt64(&stats.fetches, fetches)
	}

	stats.report(t, "CRL cache burst")
}

// TestLoadCRLCacheFailedFetchNeverCached verifies that under concurrent load,
// a failed singleflight leader shares the error with all waiters, does not cache,
// and a subsequent wave of requests retries successfully and populates the cache.
func TestLoadCRLCacheFailedFetchNeverCached(t *testing.T) {
	caCert, caKey := generateTestCert(t, true, nil, nil, nil)
	validCRLBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

	var callCount int64
	cache := &CRLCache{
		entries: make(map[string]*crlEntry),
		fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
			n := atomic.AddInt64(&callCount, 1)
			time.Sleep(10 * time.Millisecond)
			if n == 1 {
				return nil, fmt.Errorf("transient error")
			}
			return validCRLBytes, nil
		},
	}

	const url = "http://example.com/crl"
	const goroutines = 100

	// Wave 1: first fetch fails, all concurrent callers share the error.
	type callResult struct {
		crl *x509.RevocationList
		err error
	}
	wave1 := make([]callResult, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			crl, err := cache.getOrFetchCRL(context.Background(), url, caCert)
			wave1[idx] = callResult{crl: crl, err: err}
		}(i)
	}
	wg.Wait()

	// All wave 1 callers should have failed.
	for i, r := range wave1 {
		if r.err == nil {
			t.Fatalf("wave 1, caller %d: expected error, got success", i)
		}
		if r.crl != nil {
			t.Fatalf("wave 1, caller %d: expected nil CRL on error", i)
		}
	}

	// Cache must be empty after failure.
	cache.mu.RLock()
	_, cached := cache.entries[url]
	cache.mu.RUnlock()
	if cached {
		t.Fatal("failed CRL fetch was cached")
	}

	// Wave 2: retry succeeds and populates the cache.
	wave2 := make([]callResult, goroutines)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			crl, err := cache.getOrFetchCRL(context.Background(), url, caCert)
			wave2[idx] = callResult{crl: crl, err: err}
		}(i)
	}
	wg.Wait()

	// All wave 2 callers should succeed.
	for i, r := range wave2 {
		if r.err != nil {
			t.Fatalf("wave 2, caller %d: unexpected error: %v", i, r.err)
		}
		if r.crl == nil {
			t.Fatalf("wave 2, caller %d: got nil CRL", i)
		}
	}

	// Cache must now contain the CRL.
	cache.mu.RLock()
	entry, ok := cache.entries[url]
	cache.mu.RUnlock()
	if !ok || entry == nil || entry.crl == nil {
		t.Fatal("successful CRL fetch did not populate cache")
	}

	t.Logf("CRL fail-retry: wave1 all failed, wave2 all succeeded, cache populated (total fetch calls: %d)", atomic.LoadInt64(&callCount))
}

// TestLoadCRLCacheWrongIssuerNeverShared verifies that under concurrent load,
// a CRL signed by the wrong issuer is never returned to any caller.
func TestLoadCRLCacheWrongIssuerNeverShared(t *testing.T) {
	correctCA, _ := generateTestCert(t, true, nil, nil, nil)
	wrongCA, wrongKey := generateTestCert(t, true, nil, nil, nil)
	wrongIssuerCRL := createTestCRL(t, wrongCA, wrongKey, time.Now().Add(time.Hour))

	cache := &CRLCache{
		entries: make(map[string]*crlEntry),
		fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
			time.Sleep(10 * time.Millisecond)
			return wrongIssuerCRL, nil
		},
	}

	const goroutines = 100
	type callResult struct {
		crl *x509.RevocationList
		err error
	}
	results := make([]callResult, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", correctCA)
			results[idx] = callResult{crl: crl, err: err}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.err == nil {
			t.Fatalf("caller %d: expected error for wrong-issuer CRL, got success", i)
		}
		if r.crl != nil {
			t.Fatalf("caller %d: expected nil CRL for wrong issuer", i)
		}
	}

	if len(cache.entries) != 0 {
		t.Fatal("wrong-issuer CRL was cached")
	}
}

// TestLoadCRLCacheMixedURLs simulates concurrent requests for different CRL URLs
// to verify no cross-contamination between URLs.
func TestLoadCRLCacheMixedURLs(t *testing.T) {
	caCert, caKey := generateTestCert(t, true, nil, nil, nil)

	nextUpdates := map[string]time.Duration{
		"http://example.com/crl-a": 1 * time.Hour,
		"http://example.com/crl-b": 2 * time.Hour,
		"http://example.com/crl-c": 3 * time.Hour,
	}
	cache := &CRLCache{
		entries: make(map[string]*crlEntry),
		fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
			time.Sleep(15 * time.Millisecond)
			offset := nextUpdates[url]
			return createTestCRL(t, caCert, caKey, time.Now().Add(offset)), nil
		},
	}

	const goroutinesPerURL = 100
	urls := []string{"http://example.com/crl-a", "http://example.com/crl-b", "http://example.com/crl-c"}

	type callResult struct {
		url string
		crl *x509.RevocationList
		err error
	}

	results := make([]callResult, 0, len(urls)*goroutinesPerURL)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, url := range urls {
		for i := 0; i < goroutinesPerURL; i++ {
			wg.Add(1)
			go func(u string) {
				defer wg.Done()
				crl, err := cache.getOrFetchCRL(context.Background(), u, caCert)
				mu.Lock()
				results = append(results, callResult{url: u, crl: crl, err: err})
				mu.Unlock()
			}(url)
		}
	}
	wg.Wait()

	// Assert from parent goroutine.
	refCRLs := make(map[string]*x509.RevocationList)
	for _, res := range results {
		if res.err != nil {
			t.Fatalf("URL %s: unexpected error: %v", res.url, res.err)
		}
		if res.crl == nil {
			t.Fatalf("URL %s: got nil CRL", res.url)
		}
		if ref, ok := refCRLs[res.url]; ok {
			if !ref.NextUpdate.Equal(res.crl.NextUpdate) {
				t.Fatalf("URL %s: callers got different CRLs", res.url)
			}
		} else {
			refCRLs[res.url] = res.crl
		}
	}

	// Different URLs got different CRLs.
	if refCRLs[urls[0]].NextUpdate.Equal(refCRLs[urls[1]].NextUpdate) {
		t.Fatal("different URLs returned the same CRL (a == b)")
	}
	if refCRLs[urls[1]].NextUpdate.Equal(refCRLs[urls[2]].NextUpdate) {
		t.Fatal("different URLs returned the same CRL (b == c)")
	}
}
