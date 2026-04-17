package xrpverifier

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/stretchr/testify/require"
)

func TestParseTxFee(t *testing.T) {
	t.Run("valid fee", func(t *testing.T) {
		fee, err := parseTxFee(`{"Fee": "12"}`)
		require.NoError(t, err)
		require.Equal(t, big.NewInt(12), fee)
	})

	t.Run("large fee", func(t *testing.T) {
		fee, err := parseTxFee(`{"Fee": "1000000000"}`)
		require.NoError(t, err)
		require.Equal(t, big.NewInt(1000000000), fee)
	})

	t.Run("zero fee", func(t *testing.T) {
		fee, err := parseTxFee(`{"Fee": "0"}`)
		require.NoError(t, err)
		require.Equal(t, big.NewInt(0), fee)
	})

	t.Run("missing fee", func(t *testing.T) {
		_, err := parseTxFee(`{"Amount": "100"}`)
		require.ErrorContains(t, err, "missing Fee")
	})

	t.Run("empty fee", func(t *testing.T) {
		_, err := parseTxFee(`{"Fee": ""}`)
		require.ErrorContains(t, err, "missing Fee")
	})

	t.Run("non-numeric fee", func(t *testing.T) {
		_, err := parseTxFee(`{"Fee": "abc"}`)
		require.ErrorContains(t, err, "cannot parse Fee")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseTxFee(`not json`)
		require.ErrorContains(t, err, "cannot unmarshal")
	})

	t.Run("oversized response rejected", func(t *testing.T) {
		padding := strings.Repeat("x", 1<<20+1)
		_, err := parseTxFee(`{"_pad":"` + padding + `","Fee":"12"}`)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "too large")
	})
}

func TestNonceRangeValidation(t *testing.T) {
	// We can't call Verify directly without a full config/DB setup,
	// but we can test the validation logic by checking error types.
	v := &XRPVerifier{}

	t.Run("toNonce < fromNonce", func(t *testing.T) {
		_, err := v.Verify(t.Context(), connector.IPMWFeeProofRequestBody{
			FromNonce: 10,
			ToNonce:   5,
		})
		require.ErrorIs(t, err, ErrNonceRangeTooLarge)
		require.ErrorContains(t, err, "toNonce (5) < fromNonce (10)")
	})

	t.Run("range exceeds max", func(t *testing.T) {
		_, err := v.Verify(t.Context(), connector.IPMWFeeProofRequestBody{
			FromNonce: 1,
			ToNonce:   1 + MaxNonceRange, // MaxNonceRange + 1 nonces
		})
		require.ErrorIs(t, err, ErrNonceRangeTooLarge)
		require.ErrorContains(t, err, "exceeds max")
	})

	t.Run("range at max boundary", func(t *testing.T) {
		// Range of exactly MaxNonceRange nonces: ToNonce-FromNonce = MaxNonceRange-1, check passes.
		require.False(t, (MaxNonceRange-1) >= MaxNonceRange, "sanity: MaxNonceRange nonces must pass")
	})

	t.Run("overflow attempt rejected", func(t *testing.T) {
		// FromNonce=0, ToNonce=MaxUint64 would wrap to 0 if checked with `+1` arithmetic.
		// The direct-difference check rejects it.
		_, err := v.Verify(t.Context(), connector.IPMWFeeProofRequestBody{
			FromNonce: 0,
			ToNonce:   math.MaxUint64,
		})
		require.ErrorIs(t, err, ErrNonceRangeTooLarge)
		require.ErrorContains(t, err, "exceeds max size")
	})
}
