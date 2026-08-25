package btcverifier

import (
	"context"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/stretchr/testify/require"

	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
)

// TestNewOnChainResolverRejectsMalformedURL: a Flare RPC URL that does not parse
// fails resolver construction rather than deferring to first use.
func TestNewOnChainResolverRejectsMalformedURL(t *testing.T) {
	r, err := NewOnChainResolver("http://[::1", common.HexToAddress("0xC1"))
	require.Error(t, err)
	require.Nil(t, r)
}

// TestBaseResponseNormalizesNilFields: a message with nil TokenId/Amount/MaxFee
// must yield a response whose monetary fields are never nil, so the ABI encoder
// cannot panic on a missing big.Int.
func TestBaseResponseNormalizesNilFields(t *testing.T) {
	resp := baseResponse(&payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage{
		TokenId: nil, Amount: nil, MaxFee: nil,
	})
	require.NotNil(t, resp.TokenId)
	require.Equal(t, []byte{}, resp.TokenId)
	require.Equal(t, big.NewInt(0), resp.Amount)
	require.Equal(t, big.NewInt(0), resp.MaxFee)
	require.Equal(t, big.NewInt(0), resp.ReceivedAmount)
	require.Equal(t, big.NewInt(0), resp.TransactionFee)
}

// TestCheckNetworkRejectsUnknownParams: params with no chain mapping are an
// unsupported-source misconfiguration, caught before any node probe runs.
func TestCheckNetworkRejectsUnknownParams(t *testing.T) {
	p := &countingProber{chain: "signet"}
	v := &BtcVerifier{Params: &chaincfg.Params{Net: 0xdeadbeef}, prober: p}
	err := v.checkNetwork(context.Background())
	require.ErrorIs(t, err, ErrUnsupportedSource)
	require.Equal(t, int64(0), p.calls.Load(), "must reject before probing the node")
}

// TestEqualBig covers the nil handling equalBig adds on top of big.Int.Cmp: two
// nils are equal, a nil and a value are not, and equal values compare equal.
func TestEqualBig(t *testing.T) {
	require.True(t, equalBig(nil, nil))
	require.False(t, equalBig(nil, big.NewInt(1)))
	require.False(t, equalBig(big.NewInt(1), nil))
	require.True(t, equalBig(big.NewInt(7), big.NewInt(7)))
	require.False(t, equalBig(big.NewInt(7), big.NewInt(8)))
}

// TestIsWitnessProgram covers the length and version-opcode guards: only a
// version byte (OP_0 or OP_1..OP_16) followed by a single 2..40 byte push counts.
func TestIsWitnessProgram(t *testing.T) {
	require.True(t, isWitnessProgram([]byte{0x00, 0x02, 0xaa, 0xbb}))  // OP_0 + 2-byte push, len == 2+pushLen
	require.True(t, isWitnessProgram([]byte{0x51, 0x02, 0xaa, 0xbb}))  // OP_1 (v1) + 2-byte push
	require.False(t, isWitnessProgram([]byte{0x00, 0x01, 0x02}))       // too short (<4)
	require.False(t, isWitnessProgram(make([]byte, 43)))               // too long (>42)
	require.False(t, isWitnessProgram([]byte{0x61, 0x02, 0xaa, 0xbb})) // bad version opcode
	require.False(t, isWitnessProgram([]byte{0x00, 0x05, 0xaa, 0xbb})) // push len disagrees with length
}

// TestExpectedChain maps every supported chaincfg network to its
// getblockchaininfo name and rejects an unknown network.
func TestExpectedChain(t *testing.T) {
	for _, c := range []struct {
		params *chaincfg.Params
		want   string
	}{
		{&chaincfg.MainNetParams, "main"},
		{&chaincfg.TestNet3Params, "test"},
		{&chaincfg.SigNetParams, "signet"},
		{&chaincfg.RegressionNetParams, "regtest"},
	} {
		got, ok := expectedChain(c.params)
		require.True(t, ok)
		require.Equal(t, c.want, got)
	}
	_, ok := expectedChain(&chaincfg.Params{Net: 0xdeadbeef})
	require.False(t, ok)
}

// TestCheckMessageConsistency binds a decoded message to the request and resolved
// batch; each field that disagrees is a C-chain inconsistency (fail closed).
func TestCheckMessageConsistency(t *testing.T) {
	const paymentID, batchPaymentID = uint64(11), uint64(900000)
	base := sampleMsg("addr", 1000, paymentID, batchPaymentID, 7, 0)
	base.AccountAddress = testAccount

	require.NoError(t, checkMessageConsistency(&base, testSourceID, testAccount, paymentID, batchPaymentID))

	t.Run("wrong sourceId", func(t *testing.T) {
		m := base
		require.Error(t, checkMessageConsistency(&m, common.HexToHash("0x99"), testAccount, paymentID, batchPaymentID))
	})
	t.Run("wrong account", func(t *testing.T) {
		m := base
		require.Error(t, checkMessageConsistency(&m, testSourceID, "other", paymentID, batchPaymentID))
	})
	t.Run("wrong paymentId", func(t *testing.T) {
		m := base
		require.Error(t, checkMessageConsistency(&m, testSourceID, testAccount, paymentID+1, batchPaymentID))
	})
	t.Run("wrong batchPaymentId", func(t *testing.T) {
		m := base
		require.Error(t, checkMessageConsistency(&m, testSourceID, testAccount, paymentID, batchPaymentID+1))
	})
}

// TestRecipientScript: a valid instruction recipient decodes to its scriptPubKey;
// an address that does not decode is a C-chain inconsistency (fail closed).
func TestRecipientScript(t *testing.T) {
	v := &BtcVerifier{Params: testParams}
	addr, want := recipient(t)

	got, err := v.recipientScript(addr)
	require.NoError(t, err)
	require.Equal(t, want, got)

	_, err = v.recipientScript("not-a-bitcoin-address")
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

// TestSelectPayment covers not-found, a benign reissue (repeats that agree), and a
// conflicting reissue (repeats that disagree on a load-bearing field).
func TestSelectPayment(t *testing.T) {
	a := sampleMsg("addr", 1000, 11, 900000, 7, 0)
	agree := sampleMsg("addr", 1000, 11, 900000, 7, 0)    // same paymentId, identical fields
	conflict := sampleMsg("addr", 2000, 11, 900000, 7, 0) // same paymentId, different amount
	req := fdc2.IPMWPaymentStatusRequestBody{PaymentId: 11}

	t.Run("not found", func(t *testing.T) {
		_, err := selectPayment([]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage{&a}, fdc2.IPMWPaymentStatusRequestBody{PaymentId: 99})
		require.ErrorIs(t, err, paymentdb.ErrRecordNotFound)
	})
	t.Run("benign reissue resolves", func(t *testing.T) {
		got, err := selectPayment([]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage{&a, &agree}, req)
		require.NoError(t, err)
		require.Equal(t, uint64(11), got.PaymentId)
	})
	t.Run("conflicting reissue fails closed", func(t *testing.T) {
		_, err := selectPayment([]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage{&a, &conflict}, req)
		require.ErrorIs(t, err, paymentdb.ErrDatabase)
	})
}
