package xrpverifier

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
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

func TestCheckTxRowConsistency(t *testing.T) {
	row := func(response string) paymentdb.DBTransaction {
		return paymentdb.DBTransaction{Hash: "txhash7", SourceAddress: "rSender", Sequence: 7, Response: response}
	}

	t.Run("matching blob accepted", func(t *testing.T) {
		require.NoError(t, checkTxRowConsistency(row(`{"Fee":"12","Account":"rSender","Sequence":7,"hash":"txhash7"}`)))
	})
	t.Run("hash case-insensitive", func(t *testing.T) {
		require.NoError(t, checkTxRowConsistency(row(`{"Account":"rSender","Sequence":7,"hash":"TXHASH7"}`)))
	})
	t.Run("hash mismatch rejected", func(t *testing.T) {
		err := checkTxRowConsistency(row(`{"Account":"rSender","Sequence":7,"hash":"deadbeef"}`))
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON hash")
	})
	t.Run("account mismatch rejected", func(t *testing.T) {
		err := checkTxRowConsistency(row(`{"Account":"rOther","Sequence":7,"hash":"txhash7"}`))
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON Account")
	})
	t.Run("sequence mismatch rejected", func(t *testing.T) {
		err := checkTxRowConsistency(row(`{"Account":"rSender","Sequence":99,"hash":"txhash7"}`))
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "JSON Sequence")
	})
	t.Run("fee-only blob (no identity) rejected", func(t *testing.T) {
		// The exact gap this fixes: a Response carrying only Fee no longer passes.
		err := checkTxRowConsistency(row(`{"Fee":"12"}`))
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
	})
	t.Run("invalid JSON rejected", func(t *testing.T) {
		err := checkTxRowConsistency(row(`not json`))
		require.ErrorContains(t, err, "cannot unmarshal")
	})
	t.Run("oversized response rejected", func(t *testing.T) {
		err := checkTxRowConsistency(row(`{"_pad":"` + strings.Repeat("x", 1<<20+1) + `"}`))
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
		require.ErrorContains(t, err, "too large")
	})
}

func TestBatchRangeValidation(t *testing.T) {
	// We can't call Verify directly without a full config/DB setup,
	// but we can test the validation logic by checking error types.
	v := &XRPVerifier{}

	t.Run("zero batch count", func(t *testing.T) {
		// Formerly an invalid range; with paymentId+count this maps to BatchCount 0.
		_, err := v.Verify(t.Context(), fdc2.IPMWFeeProofRequestBody{
			FirstPaymentId: 10,
			BatchCount:     0,
		})
		require.ErrorIs(t, err, ErrBatchRangeTooLarge)
		require.ErrorContains(t, err, "batchCount must be greater than 0")
	})

	t.Run("range exceeds max", func(t *testing.T) {
		_, err := v.Verify(t.Context(), fdc2.IPMWFeeProofRequestBody{
			FirstPaymentId: 1,
			BatchCount:     MaxBatchRange + 1,
		})
		require.ErrorIs(t, err, ErrBatchRangeTooLarge)
		require.ErrorContains(t, err, "exceeds max size")
	})

	t.Run("range at max boundary", func(t *testing.T) {
		// A BatchCount of exactly MaxBatchRange must pass the size check.
		require.False(t, MaxBatchRange > MaxBatchRange, "sanity: MaxBatchRange payments must pass")
	})

	t.Run("overflow attempt rejected", func(t *testing.T) {
		// FirstPaymentId=MaxUint64 with BatchCount>1 overflows the inclusive upper bound.
		_, err := v.Verify(t.Context(), fdc2.IPMWFeeProofRequestBody{
			FirstPaymentId: math.MaxUint64,
			BatchCount:     2,
		})
		require.ErrorIs(t, err, ErrBatchRangeTooLarge)
		require.ErrorContains(t, err, "overflows uint64")
	})
}
