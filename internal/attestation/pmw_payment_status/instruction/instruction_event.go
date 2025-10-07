package instruction

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
)

const EventNameTeeInstructionsSent = "TeeInstructionsSent"

func GetTeeInstructionsSentEventSignature(abiDef abi.ABI) (string, error) {
	event, exists := abiDef.Events[EventNameTeeInstructionsSent]
	if !exists {
		return "", fmt.Errorf("ABI does not contain event %s", EventNameTeeInstructionsSent)
	}
	return event.ID.Hex(), nil
}

func DecodeTeeInstructionsSentEventData(log *types.Log, teeABI abi.ABI) (*payment.ITeePaymentsPaymentInstructionMessage, error) {
	eventData, err := abiDecodeEventData[teeextensionregistry.TeeExtensionRegistryTeeInstructionsSent](
		teeABI,
		EventNameTeeInstructionsSent,
		log.Data,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot decode event %s: %w", EventNameTeeInstructionsSent, err)
	}
	var message payment.ITeePaymentsPaymentInstructionMessage
	err = structs.DecodeTo(payment.MessageArguments[op.Pay], eventData.Message, &message)
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
