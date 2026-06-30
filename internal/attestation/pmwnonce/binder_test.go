package pmwnonce_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/teepayments"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwnonce"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/stretchr/testify/require"
)

// stubCaller is a test double for the TeePayments getInitialNonce binding. It
// records call count so cache behaviour can be asserted.
type stubCaller struct {
	nonce   uint64
	err     error
	calls   int
	lastAcc teepayments.ITeePaymentsBasePMWMultisigAccount
}

func (s *stubCaller) GetInitialNonce(_ *bind.CallOpts, account teepayments.ITeePaymentsBasePMWMultisigAccount) (uint64, error) {
	s.calls++
	s.lastAcc = account
	return s.nonce, s.err
}

func TestVerifySequence(t *testing.T) {
	sourceID := common.HexToHash("0x1")
	const account = "rSender"

	t.Run("matches deterministic sequence", func(t *testing.T) {
		c := &stubCaller{nonce: 100}
		b := pmwnonce.NewBinder(c)
		// sequence = initialNonce + paymentId - 1 = 100 + 5 - 1 = 104
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, account, 5, 104))
	})

	t.Run("paymentId 1 maps to initialNonce", func(t *testing.T) {
		c := &stubCaller{nonce: 100}
		b := pmwnonce.NewBinder(c)
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, account, 1, 100))
	})

	t.Run("mismatch fails closed", func(t *testing.T) {
		c := &stubCaller{nonce: 100}
		b := pmwnonce.NewBinder(c)
		err := b.VerifySequence(context.Background(), sourceID, account, 5, 999)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "XRP sequence mismatch")
	})

	t.Run("paymentId zero rejected", func(t *testing.T) {
		c := &stubCaller{nonce: 100}
		b := pmwnonce.NewBinder(c)
		err := b.VerifySequence(context.Background(), sourceID, account, 0, 100)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "paymentId must be >= 1")
	})

	t.Run("overflow rejected", func(t *testing.T) {
		c := &stubCaller{nonce: ^uint64(0)} // MaxUint64
		b := pmwnonce.NewBinder(c)
		err := b.VerifySequence(context.Background(), sourceID, account, 2, 0)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "overflows uint64")
	})

	t.Run("caller error fails closed", func(t *testing.T) {
		c := &stubCaller{err: errors.New("rpc down")}
		b := pmwnonce.NewBinder(c)
		err := b.VerifySequence(context.Background(), sourceID, account, 5, 104)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "cannot read initialNonce")
	})
}

func TestBinderCloseNilClient(t *testing.T) {
	// A binder built from a caller directly (no RPC client) closes cleanly and is
	// safe to close more than once.
	b := pmwnonce.NewBinder(&stubCaller{})
	require.NoError(t, b.Close())
	require.NoError(t, b.Close())
}

func TestNewOnChainBinderBadURL(t *testing.T) {
	// An unsupported URL scheme fails at dial time (fail closed at construction)
	// rather than booting clean and failing every request.
	b, err := pmwnonce.NewOnChainBinder("ftp://not-a-flare-node", common.HexToAddress("0xC1"))
	require.Error(t, err)
	require.Nil(t, b)
	require.ErrorContains(t, err, "cannot connect to Flare node")
}

func TestInitialNonceCache(t *testing.T) {
	sourceID := common.HexToHash("0x1")

	t.Run("same account fetched once", func(t *testing.T) {
		c := &stubCaller{nonce: 100}
		b := pmwnonce.NewBinder(c)
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "rA", 1, 100))
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "rA", 2, 101))
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "rA", 3, 102))
		require.Equal(t, 1, c.calls, "initialNonce is immutable; it must be cached per account")
	})

	t.Run("distinct accounts fetched separately", func(t *testing.T) {
		c := &stubCaller{nonce: 100}
		b := pmwnonce.NewBinder(c)
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "rA", 1, 100))
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "rB", 1, 100))
		require.Equal(t, 2, c.calls)
		require.Equal(t, "rB", c.lastAcc.AccountAddress)
		require.Equal(t, sourceID, common.Hash(c.lastAcc.SourceId))
	})

	t.Run("error is not cached", func(t *testing.T) {
		c := &stubCaller{err: errors.New("rpc down")}
		b := pmwnonce.NewBinder(c)
		require.Error(t, b.VerifySequence(context.Background(), sourceID, "rA", 1, 100))
		c.err = nil
		c.nonce = 100
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "rA", 1, 100))
		require.Equal(t, 2, c.calls, "a failed lookup must not poison the cache")
	})

	t.Run("least-recently-used entry is evicted at capacity", func(t *testing.T) {
		// capEntries mirrors the unexported maxInitialNonceCacheEntries.
		const capEntries = 100_000
		c := &stubCaller{nonce: 100}
		b := pmwnonce.NewBinder(c)

		// Insert the target first so it is the least-recently-used entry, then fill
		// the cache to capacity with distinct accounts, which evicts the target.
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "target", 1, 100))
		for i := 0; i < capEntries; i++ {
			require.NoError(t, b.VerifySequence(context.Background(), sourceID, fmt.Sprintf("acct-%d", i), 1, 100))
		}
		require.Equal(t, capEntries+1, c.calls, "each distinct account fetched exactly once")

		// A still-cached account is not re-fetched; the evicted target is.
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "acct-0", 1, 100))
		require.Equal(t, capEntries+1, c.calls, "recently-used account stays cached")
		require.NoError(t, b.VerifySequence(context.Background(), sourceID, "target", 1, 100))
		require.Equal(t, capEntries+2, c.calls, "evicted account is re-fetched")
	})
}
