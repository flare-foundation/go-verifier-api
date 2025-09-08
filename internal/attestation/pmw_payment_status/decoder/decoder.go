package decoder

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

const EventNameTeeInstructionsSent = "TeeInstructionsSent"

func GetTeeInstructionsSentEventSignature(abiDef abi.ABI) (string, error) {
	event, exists := abiDef.Events[EventNameTeeInstructionsSent]
	if !exists {
		return "", fmt.Errorf("event %s not found", EventNameTeeInstructionsSent)
	}
	return event.ID.Hex(), nil
}

func DecodeTeeInstructionsSentEventData(log *types.Log, teeABI abi.ABI) (*payment.ITeePaymentsPaymentInstructionMessage, error) {
	eventData, err := utils.ABIDecodeEventData[teeextensionregistry.TeeExtensionRegistryTeeInstructionsSent](
		teeABI,
		EventNameTeeInstructionsSent,
		log.Data,
	)
	if err != nil {
		return nil, err
	}
	var message payment.ITeePaymentsPaymentInstructionMessage
	err = structs.DecodeTo(payment.MessageArguments[op.Pay], eventData.Message, &message)
	if err != nil {
		return nil, err
	}
	return &message, nil
}
