package db

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/stretchr/testify/require"
)

func TestCheckInstructionConsistency(t *testing.T) {
	sourceID := common.HexToHash("0x1")
	const sender = "rSender"
	// paymentId is the small request key; sequence is the (distinct) XRP Sequence
	// carried in the message Nonce. Consistency binds on PaymentId, never Nonce.
	const paymentId = uint64(7)
	const sequence = uint64(42)

	base := func() *payments.ITeePaymentsPaymentInstructionMessage {
		return &payments.ITeePaymentsPaymentInstructionMessage{
			SourceId:      sourceID,
			SenderAddress: sender,
			Nonce:         sequence,
			PaymentId:     paymentId,
		}
	}

	t.Run("consistent message passes", func(t *testing.T) {
		require.NoError(t, CheckInstructionConsistency(base(), sourceID, sender, paymentId))
	})
	t.Run("SourceId mismatch fails closed", func(t *testing.T) {
		msg := base()
		msg.SourceId = common.HexToHash("0x2")
		err := CheckInstructionConsistency(msg, sourceID, sender, paymentId)
		require.ErrorIs(t, err, ErrDatabase)
		require.ErrorContains(t, err, "event SourceId")
	})
	t.Run("SenderAddress mismatch fails closed", func(t *testing.T) {
		msg := base()
		msg.SenderAddress = "rOther"
		err := CheckInstructionConsistency(msg, sourceID, sender, paymentId)
		require.ErrorIs(t, err, ErrDatabase)
		require.ErrorContains(t, err, "event SenderAddress")
	})
	t.Run("PaymentId mismatch fails closed", func(t *testing.T) {
		msg := base()
		msg.PaymentId = paymentId + 1
		err := CheckInstructionConsistency(msg, sourceID, sender, paymentId)
		require.ErrorIs(t, err, ErrDatabase)
		require.ErrorContains(t, err, "event PaymentId")
	})
	t.Run("error wraps ErrDatabase (maps to 503)", func(t *testing.T) {
		msg := base()
		msg.PaymentId = 0
		require.True(t, errors.Is(CheckInstructionConsistency(msg, sourceID, sender, paymentId), ErrDatabase))
	})
}
