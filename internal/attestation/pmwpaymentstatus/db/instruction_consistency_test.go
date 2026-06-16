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
	const nonce = uint64(42)

	base := func() *payments.ITeePaymentsPaymentInstructionMessage {
		return &payments.ITeePaymentsPaymentInstructionMessage{
			SourceId:      sourceID,
			SenderAddress: sender,
			Nonce:         nonce,
		}
	}

	t.Run("consistent message passes", func(t *testing.T) {
		require.NoError(t, CheckInstructionConsistency(base(), sourceID, sender, nonce))
	})
	t.Run("SourceId mismatch fails closed", func(t *testing.T) {
		msg := base()
		msg.SourceId = common.HexToHash("0x2")
		err := CheckInstructionConsistency(msg, sourceID, sender, nonce)
		require.ErrorIs(t, err, ErrDatabase)
		require.ErrorContains(t, err, "event SourceId")
	})
	t.Run("SenderAddress mismatch fails closed", func(t *testing.T) {
		msg := base()
		msg.SenderAddress = "rOther"
		err := CheckInstructionConsistency(msg, sourceID, sender, nonce)
		require.ErrorIs(t, err, ErrDatabase)
		require.ErrorContains(t, err, "event SenderAddress")
	})
	t.Run("Nonce mismatch fails closed", func(t *testing.T) {
		msg := base()
		msg.Nonce = nonce + 1
		err := CheckInstructionConsistency(msg, sourceID, sender, nonce)
		require.ErrorIs(t, err, ErrDatabase)
		require.ErrorContains(t, err, "event Nonce")
	})
	t.Run("error wraps ErrDatabase (maps to 503)", func(t *testing.T) {
		msg := base()
		msg.Nonce = 0
		require.True(t, errors.Is(CheckInstructionConsistency(msg, sourceID, sender, nonce), ErrDatabase))
	})
}
