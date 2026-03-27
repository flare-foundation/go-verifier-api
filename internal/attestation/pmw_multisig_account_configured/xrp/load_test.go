//go:build load

package xrpverifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// TestLoadMultisigConcurrentVerify simulates concurrent verification requests
// against a mock XRP RPC endpoint.
func TestLoadMultisigConcurrentVerify(t *testing.T) {
	testAccounts := createTestAccounts(t, 3)

	accountInfoResp := makeAccountInfo(t,
		makeSignerList(t,
			[]string{testAccounts[0].Address, testAccounts[1].Address, testAccounts[2].Address},
			[]uint16{1, 1, 1}, 2),
		accountFlags(t, true, false, false, false),
		"", 9999,
	)

	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(accountInfoResp)
	}))
	defer server.Close()

	v := &XRPVerifier{
		Config: &config.PMWMultisigAccountConfig{},
		Client: client.NewClient(server.URL),
	}

	req := makeIPMWMultisigAccountConfiguredRequestBody(t, "rTestAccount",
		[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey, testAccounts[2].PubKey}, 2)

	const (
		concurrency = 100
		rounds      = 20
	)

	type callResult struct {
		resp connector.IPMWMultisigAccountConfiguredResponseBody
		err  error
	}

	var latencies []time.Duration
	var mu sync.Mutex

	for round := 0; round < rounds; round++ {
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				resp, err := v.Verify(context.Background(), req)
				elapsed := time.Since(start)
				mu.Lock()
				latencies = append(latencies, elapsed)
				mu.Unlock()
				results[idx] = callResult{resp: resp, err: err}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			if r.err != nil {
				t.Fatalf("round %d, caller %d: unexpected error: %v", round, i, r.err)
			}
			if r.resp.Status != 0 {
				t.Fatalf("round %d, caller %d: expected status OK (0), got %d", round, i, r.resp.Status)
			}
			if r.resp.Sequence != 9999 {
				t.Fatalf("round %d, caller %d: expected sequence 9999, got %d", round, i, r.resp.Sequence)
			}
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	n := len(latencies)
	t.Logf("Multisig concurrent verify: n=%d, p50=%v, p95=%v, p99=%v, total RPC calls=%d",
		n, latencies[n*50/100], latencies[n*95/100], latencies[n*99/100],
		atomic.LoadInt64(&requestCount))
}

// TestLoadMultisigSlowUpstream verifies timeout behavior when the XRP RPC is slow.
func TestLoadMultisigSlowUpstream(t *testing.T) {
	testAccounts := createTestAccounts(t, 2)

	accountInfoResp := makeAccountInfo(t,
		makeSignerList(t,
			[]string{testAccounts[0].Address, testAccounts[1].Address},
			[]uint16{1, 1}, 2),
		accountFlags(t, true, false, false, false),
		"", 1234,
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // simulate slow upstream
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(accountInfoResp)
	}))
	defer server.Close()

	v := &XRPVerifier{
		Config: &config.PMWMultisigAccountConfig{},
		Client: client.NewClient(server.URL),
	}

	req := makeIPMWMultisigAccountConfiguredRequestBody(t, "rTestAccount",
		[][]byte{testAccounts[0].PubKey, testAccounts[1].PubKey}, 2)

	const concurrency = 6
	type callResult struct {
		resp    connector.IPMWMultisigAccountConfiguredResponseBody
		err     error
		elapsed time.Duration
	}

	results := make([]callResult, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			start := time.Now()
			resp, err := v.Verify(context.Background(), req)
			results[idx] = callResult{resp: resp, err: err, elapsed: time.Since(start)}
		}(i)
	}
	wg.Wait()

	// All should complete: 3s upstream delay is within the 4s client timeout.
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("caller %d: expected success under slow upstream, got error after %v: %v", i, r.elapsed, r.err)
		}
		if r.resp.Status != 0 {
			t.Fatalf("caller %d: expected OK, got status %d", i, r.resp.Status)
		}
		t.Logf("caller %d: elapsed=%v", i, r.elapsed)
	}
}
