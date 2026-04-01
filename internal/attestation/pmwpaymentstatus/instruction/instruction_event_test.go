package instruction_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/stretchr/testify/require"
)

func loadTestABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	require.NoError(t, err)
	return parsed
}

func encodeTestEvent(t *testing.T, teeABI abi.ABI, msg payment.ITeePaymentsPaymentInstructionMessage) []byte {
	t.Helper()
	msgArg := payment.MessageArguments[op.Pay]
	msgBytes, err := structs.Encode(msgArg, msg)
	require.NoError(t, err)

	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]teeextensionregistry.ITeeMachineRegistryTeeMachine{},
		[32]byte{},
		[32]byte{},
		msgBytes,
		[]common.Address{},
		uint64(0),
		common.Address{},
		big.NewInt(0),
	)
	require.NoError(t, err)
	return data
}

func TestTeeInstructionsSentEventSignature(t *testing.T) {
	t.Run("valid ABI returns event hash", func(t *testing.T) {
		teeABI := loadTestABI(t)
		hash, err := instruction.TeeInstructionsSentEventSignature(teeABI)
		require.NoError(t, err)
		require.NotEmpty(t, hash)
		require.True(t, strings.HasPrefix(hash, "0x"), "expected hex prefix")
	})

	t.Run("empty ABI returns error", func(t *testing.T) {
		emptyABI := abi.ABI{}
		_, err := instruction.TeeInstructionsSentEventSignature(emptyABI)
		require.ErrorContains(t, err, "ABI does not contain event")
	})
}

func TestDecodeTeeInstructionsSentEventData(t *testing.T) {
	teeABI := loadTestABI(t)

	t.Run("valid event data decodes correctly", func(t *testing.T) {
		msg := payment.ITeePaymentsPaymentInstructionMessage{
			SenderAddress:    "rSender",
			RecipientAddress: "rRecipient",
			Amount:           big.NewInt(1000),
			MaxFee:           big.NewInt(50),
			TokenId:          []byte{},
			FeeSchedule:      []byte{},
			Nonce:            42,
			SubNonce:         42,
		}
		eventData := encodeTestEvent(t, teeABI, msg)

		log := &ethtypes.Log{Data: eventData}
		decoded, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay)
		require.NoError(t, err)
		require.NotNil(t, decoded)
		require.Equal(t, "rSender", decoded.SenderAddress)
		require.Equal(t, "rRecipient", decoded.RecipientAddress)
		require.Equal(t, big.NewInt(1000), decoded.Amount)
		require.Equal(t, big.NewInt(50), decoded.MaxFee)
		require.Equal(t, uint64(42), decoded.Nonce)
		require.Equal(t, uint64(42), decoded.SubNonce)
	})

	t.Run("nil log data returns error", func(t *testing.T) {
		log := &ethtypes.Log{Data: nil}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay)
		require.ErrorContains(t, err, "cannot decode event")
	})

	t.Run("empty log data returns error", func(t *testing.T) {
		log := &ethtypes.Log{Data: []byte{}}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay)
		require.ErrorContains(t, err, "cannot decode event")
	})

	t.Run("malformed log data returns error", func(t *testing.T) {
		log := &ethtypes.Log{Data: []byte("not-abi-encoded")}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay)
		require.ErrorContains(t, err, "cannot decode event")
	})

	t.Run("truncated log data returns error", func(t *testing.T) {
		msg := payment.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender",
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(50),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
		}
		eventData := encodeTestEvent(t, teeABI, msg)
		// Truncate to half.
		log := &ethtypes.Log{Data: eventData[:len(eventData)/2]}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay)
		require.Error(t, err)
	})

	t.Run("valid event with corrupt message payload returns error", func(t *testing.T) {
		// Build a valid event but with garbage in the Message field.
		eventABI := teeABI.Events["TeeInstructionsSent"]
		data, err := eventABI.Inputs.NonIndexed().Pack(
			[]teeextensionregistry.ITeeMachineRegistryTeeMachine{},
			[32]byte{},
			[32]byte{},
			[]byte("not-a-valid-payment-message"), // corrupt message
			[]common.Address{},
			uint64(0),
			common.Address{},
			big.NewInt(0),
		)
		require.NoError(t, err)

		log := &ethtypes.Log{Data: data}
		_, err = instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay)
		require.ErrorContains(t, err, "cannot decode TeeInstructionsSent message arguments")
	})

	t.Run("wrong ABI returns error", func(t *testing.T) {
		msg := payment.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender",
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(50),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
		}
		eventData := encodeTestEvent(t, teeABI, msg)
		log := &ethtypes.Log{Data: eventData}

		emptyABI := abi.ABI{}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, emptyABI, op.Pay)
		require.ErrorContains(t, err, "cannot decode event")
	})
}
