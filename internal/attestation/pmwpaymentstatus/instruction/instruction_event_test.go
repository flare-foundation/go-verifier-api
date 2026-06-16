package instruction_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/instructions"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/stretchr/testify/require"
)

func loadTestABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(instructions.InstructionsMetaData.ABI))
	require.NoError(t, err)
	return parsed
}

func encodeTestEvent(t *testing.T, teeABI abi.ABI, opType common.Hash, msg payments.ITeePaymentsPaymentInstructionMessage) []byte {
	t.Helper()
	msgArg := payments.MessageArguments[op.Pay]
	msgBytes, err := structs.Encode(msgArg, msg)
	require.NoError(t, err)
	opCommand, err := convert.StringToCommonHash(string(op.Pay))
	require.NoError(t, err)

	eventABI := teeABI.Events["TeeInstructionsSent"]
	data, err := eventABI.Inputs.NonIndexed().Pack(
		[]instructions.IMachineManagerTeeMachine{},
		[32]byte(opType),
		[32]byte(opCommand),
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
	opType := common.HexToHash("0xAA")

	t.Run("valid event data decodes correctly", func(t *testing.T) {
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SenderAddress:    "rSender",
			RecipientAddress: "rRecipient",
			Amount:           big.NewInt(1000),
			MaxFee:           big.NewInt(50),
			TokenId:          []byte{},
			FeeSchedule:      []byte{},
			Nonce:            42,
			SubNonce:         42,
		}
		eventData := encodeTestEvent(t, teeABI, opType, msg)

		log := &ethtypes.Log{Data: eventData}
		decoded, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, opType)
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
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, opType)
		require.ErrorContains(t, err, "cannot decode event")
	})

	t.Run("empty log data returns error", func(t *testing.T) {
		log := &ethtypes.Log{Data: []byte{}}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, opType)
		require.ErrorContains(t, err, "cannot decode event")
	})

	t.Run("malformed log data returns error", func(t *testing.T) {
		log := &ethtypes.Log{Data: []byte("not-abi-encoded")}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, opType)
		require.ErrorContains(t, err, "cannot decode event")
	})

	t.Run("truncated log data returns error", func(t *testing.T) {
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender",
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(50),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
		}
		eventData := encodeTestEvent(t, teeABI, opType, msg)
		// Truncate to half.
		log := &ethtypes.Log{Data: eventData[:len(eventData)/2]}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, opType)
		require.Error(t, err)
	})

	t.Run("valid event with corrupt message payload returns error", func(t *testing.T) {
		// Build a valid wrapper (matching op fields) but with garbage in the Message
		// field, so decoding reaches the message stage and fails there.
		opCommand, err := convert.StringToCommonHash(string(op.Pay))
		require.NoError(t, err)
		eventABI := teeABI.Events["TeeInstructionsSent"]
		data, err := eventABI.Inputs.NonIndexed().Pack(
			[]instructions.IMachineManagerTeeMachine{},
			[32]byte(opType),
			[32]byte(opCommand),
			[]byte("not-a-valid-payment-message"), // corrupt message
			[]common.Address{},
			uint64(0),
			common.Address{},
			big.NewInt(0),
		)
		require.NoError(t, err)

		log := &ethtypes.Log{Data: data}
		_, err = instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, opType)
		require.ErrorContains(t, err, "cannot decode TeeInstructionsSent message arguments")
	})

	t.Run("OpType mismatch fails closed", func(t *testing.T) {
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender", Amount: big.NewInt(1000), MaxFee: big.NewInt(50),
			TokenId: []byte{}, FeeSchedule: []byte{}, Nonce: 42, SubNonce: 42,
		}
		log := &ethtypes.Log{Data: encodeTestEvent(t, teeABI, opType, msg)}
		// Event was encoded with opType 0xAA; decode expecting a different opType.
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, common.HexToHash("0xBB"))
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "event OpType")
	})

	t.Run("OpCommand mismatch fails closed", func(t *testing.T) {
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender", Amount: big.NewInt(1000), MaxFee: big.NewInt(50),
			TokenId: []byte{}, FeeSchedule: []byte{}, Nonce: 42, SubNonce: 42,
		}
		// Event encoded with OpCommand=PAY; decode expecting REISSUE.
		log := &ethtypes.Log{Data: encodeTestEvent(t, teeABI, opType, msg)}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Reissue, opType)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "event OpCommand")
	})

	t.Run("unconvertible command is rejected", func(t *testing.T) {
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender", Amount: big.NewInt(1000), MaxFee: big.NewInt(50),
			TokenId: []byte{}, FeeSchedule: []byte{}, Nonce: 42, SubNonce: 42,
		}
		log := &ethtypes.Log{Data: encodeTestEvent(t, teeABI, opType, msg)}
		// A command longer than 32 bytes cannot be hashed to a Bytes32 — exercises the
		// guard between the OpType and OpCommand checks.
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Command(strings.Repeat("x", 33)), opType)
		require.ErrorContains(t, err, "cannot convert command")
	})

	t.Run("oversized log data rejected", func(t *testing.T) {
		log := &ethtypes.Log{Data: make([]byte, 1<<20+1)}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, teeABI, op.Pay, opType)
		require.ErrorContains(t, err, "event data too large")
	})

	t.Run("wrong ABI returns error", func(t *testing.T) {
		msg := payments.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: "rSender",
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(50),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
		}
		eventData := encodeTestEvent(t, teeABI, opType, msg)
		log := &ethtypes.Log{Data: eventData}

		emptyABI := abi.ABI{}
		_, err := instruction.DecodeTeeInstructionsSentEventData(log, emptyABI, op.Pay, opType)
		require.ErrorContains(t, err, "cannot decode event")
	})
}
