package instruction

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/instructions"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
)

const EventNameTeeInstructionsSent = "TeeInstructionsSent"

func TeeInstructionsSentEventSignature(abiDef abi.ABI) (string, error) {
	event, exists := abiDef.Events[EventNameTeeInstructionsSent]
	if !exists {
		return "", fmt.Errorf("ABI does not contain event %s", EventNameTeeInstructionsSent)
	}
	return event.ID.Hex(), nil
}

// maxEventDataSize caps the byte length of log.Data before ABI decoding.
// Legitimate TeeInstructionsSent events are ~1–2 KB; 1 MB matches the HTTP
// request body limit and prevents OOM from a corrupted indexer row.
const maxEventDataSize = 1 << 20 // 1 MB

func DecodeTeeInstructionsSentEventData(log *types.Log, teeABI abi.ABI, command op.Command) (*payments.ITeePaymentsPaymentInstructionMessage, error) {
	if len(log.Data) > maxEventDataSize {
		return nil, fmt.Errorf("event data too large (%d bytes, max %d)", len(log.Data), maxEventDataSize)
	}
	eventData, err := abiDecodeEventData[instructions.InstructionsTeeInstructionsSent](
		teeABI,
		EventNameTeeInstructionsSent,
		log.Data,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot decode event %s: %w", EventNameTeeInstructionsSent, err)
	}
	var message payments.ITeePaymentsPaymentInstructionMessage
	err = structs.DecodeTo(payments.MessageArguments[command], eventData.Message, &message)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s message arguments: %w", EventNameTeeInstructionsSent, err)
	}
	return &message, nil
}

func abiDecodeEventData[T any](abiObj abi.ABI, eventName string, data hexutil.Bytes) (*T, error) {
	var result T
	err := abiObj.UnpackIntoInterface(&result, eventName, data)
	if err != nil {
		return nil, fmt.Errorf("ABI unpack into %T failed for event %q: %w", result, eventName, err)
	}
	return &result, nil
}
