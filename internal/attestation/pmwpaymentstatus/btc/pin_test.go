package btcverifier

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
)

// countingProber records how many chain probes ran and can block one mid-probe
// (enter signals entry; gate releases it or ctx cancels it).
type countingProber struct {
	chain string
	err   error
	calls atomic.Int64
	enter chan struct{}
	gate  chan struct{}
}

func (p *countingProber) Chain(ctx context.Context) (string, error) {
	p.calls.Add(1)
	if p.enter != nil {
		p.enter <- struct{}{}
	}
	if p.gate != nil {
		select {
		case <-p.gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return p.chain, p.err
}

// pinVerifier builds a minimal verifier exercising only the network pin. testParams
// is SigNet, so the expected chain is "signet".
func pinVerifier(p chainProber, now func() time.Time) *BtcVerifier {
	return &BtcVerifier{Params: testParams, prober: p, now: now}
}

// TestPin_FreshVerificationSkipsProbe: a confirmation within the TTL is reused
// without another RPC.
func TestPin_FreshVerificationSkipsProbe(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	p := &countingProber{chain: "signet"}
	v := pinVerifier(p, func() time.Time { return now })

	require.NoError(t, v.ensureNetworkVerified(context.Background()))
	require.Equal(t, int64(1), p.calls.Load())

	now = now.Add(networkVerifyTTL - time.Minute)
	require.NoError(t, v.ensureNetworkVerified(context.Background()))
	require.Equal(t, int64(1), p.calls.Load(), "a fresh confirmation must not re-probe")
}

// TestPin_RepointedDetectedAfterTTL: once the TTL lapses the node is re-probed,
// so a repoint to another chain is caught.
func TestPin_RepointedDetectedAfterTTL(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	p := &countingProber{chain: "signet"}
	v := pinVerifier(p, func() time.Time { return now })
	require.NoError(t, v.ensureNetworkVerified(context.Background()))

	p.chain = "main" // repointed
	now = now.Add(networkVerifyTTL + time.Minute)
	require.ErrorIs(t, v.ensureNetworkVerified(context.Background()), ErrNetworkMismatch)
	require.Equal(t, int64(2), p.calls.Load())
}

// TestPin_NegativeResultCachedForCooldown: a failed probe is reused for exactly
// the cooldown, then re-probed.
func TestPin_NegativeResultCachedForCooldown(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	p := &countingProber{err: paymentdb.ErrDatabase} // unreachable
	v := pinVerifier(p, func() time.Time { return now })

	require.ErrorIs(t, v.ensureNetworkVerified(context.Background()), paymentdb.ErrDatabase)
	require.Equal(t, int64(1), p.calls.Load())

	now = now.Add(networkProbeCooldown - time.Second) // within cooldown
	require.ErrorIs(t, v.ensureNetworkVerified(context.Background()), paymentdb.ErrDatabase)
	require.Equal(t, int64(1), p.calls.Load(), "the negative result must be cached for the cooldown")

	now = now.Add(2 * time.Second) // past cooldown
	require.ErrorIs(t, v.ensureNetworkVerified(context.Background()), paymentdb.ErrDatabase)
	require.Equal(t, int64(2), p.calls.Load())
}

// TestPin_ConcurrentStaleRequestsProbeOnce: while one caller probes, a burst of
// stale callers fail closed with ErrNetworkUnverified rather than each probing.
func TestPin_ConcurrentStaleRequestsProbeOnce(t *testing.T) {
	enter := make(chan struct{})
	gate := make(chan struct{})
	p := &countingProber{chain: "signet", enter: enter, gate: gate}
	v := pinVerifier(p, time.Now)

	winErr := make(chan error, 1)
	go func() { winErr <- v.ensureNetworkVerified(context.Background()) }()
	<-enter // winner is inside Chain, holding the probe lock

	const waiters = 8
	var wg sync.WaitGroup
	errs := make([]error, waiters)
	for i := range waiters {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = v.ensureNetworkVerified(context.Background()) }(i)
	}
	wg.Wait()
	for _, e := range errs {
		require.ErrorIs(t, e, ErrNetworkUnverified)
	}

	close(gate)
	require.NoError(t, <-winErr)
	require.Equal(t, int64(1), p.calls.Load(), "only one probe may fire")
}

// TestPin_ClockRollbackInvalidatesFreshness: a backward wall-clock jump must not
// keep a past confirmation fresh.
func TestPin_ClockRollbackInvalidatesFreshness(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	v := pinVerifier(&countingProber{chain: "signet"}, func() time.Time { return now })
	require.NoError(t, v.ensureNetworkVerified(context.Background()))
	require.True(t, v.verifiedFresh())

	now = now.Add(-time.Hour)
	require.False(t, v.verifiedFresh())
}

// TestPin_ContextCancelInterruptsProbe: cancelling the request interrupts a
// blocked probe rather than hanging.
func TestPin_ContextCancelInterruptsProbe(t *testing.T) {
	enter := make(chan struct{})
	p := &countingProber{chain: "signet", enter: enter, gate: make(chan struct{})} // gate never closed
	v := pinVerifier(p, time.Now)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- v.ensureNetworkVerified(ctx) }()
	<-enter
	cancel()
	require.ErrorIs(t, <-errCh, context.Canceled)
}
