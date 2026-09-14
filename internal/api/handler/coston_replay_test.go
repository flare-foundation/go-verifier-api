package handler

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/stretchr/testify/require"
)

// costonPaymentStatusBody rebuilds the EXACT request body the Coston
// tee-relay-client sent (from the production log): a 3-field PMWPaymentStatus
// request — opType="F_XRP", senderAddress, paymentId=1 — with NO transactionId.
// This is the "initial data" the 4-field verifier rejected as malformed.
func costonPaymentStatusBody() []byte {
	word := func(n byte) []byte { w := make([]byte, 32); w[31] = n; return w }
	b := make([]byte, 0, 7*32)
	b = append(b, word(32)...) // offset to the struct
	op := make([]byte, 32)
	copy(op, "F_XRP")
	b = append(b, op...)       // opType (bytes32)
	b = append(b, word(96)...) // senderAddress offset — 3-field layout (was 128 when re-encoded 4-field)
	b = append(b, word(1)...)  // paymentId
	b = append(b, word(34)...) // senderAddress length = 34
	addr := make([]byte, 64)
	copy(addr, "r9nHrtRgBFvCjvvbRtmqM7C2ER4JgrYhf8")
	b = append(b, addr...) // senderAddress data (34 bytes, padded to two words)
	return b
}

// TestCostonPaymentStatusRoundTrips proves the fix: the exact 3-field body that
// Coston rejected ("malformed request body" — round-trip mismatch) now decodes
// and re-encodes byte-identically against the 3-field IPMWPaymentStatusRequestBody.
//
// REMOVE THIS TEST when the 4-field (transactionId) PMWPaymentStatus contracts
// are deployed to Coston. At that point this XRP line adopts the 4-field schema
// (go-flare-common bumped back to the transactionId version and BTC merged from
// develop-btc), and a 3-field body is no longer what the contract produces — so
// this replay becomes invalid by design.
func TestCostonPaymentStatusRoundTrips(t *testing.T) {
	arg := fdc2.AttestationTypeArguments[fdc2.PMWPaymentStatus].Request
	body := costonPaymentStatusBody()

	got, err := structs.Decode[fdc2.IPMWPaymentStatusRequestBody](arg, body)
	require.NoError(t, err, "the 3-field Coston request must round-trip (this was the production bug)")
	require.Equal(t, "r9nHrtRgBFvCjvvbRtmqM7C2ER4JgrYhf8", got.SenderAddress)
	require.Equal(t, uint64(1), got.PaymentId)

	encoded, err := structs.Encode(arg, got)
	require.NoError(t, err)
	require.Equal(t, body, encoded, "verifier now re-encodes exactly the 3-field layout the contract expects")
}
