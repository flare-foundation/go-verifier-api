package decoder

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

const EventNameTeeInstructionsSent = "TeeInstructionsSent"
const PaymentInstructionMessage = "paymentInstructionMessageStruct"

func GetTeeInstructionsSentEventSignature(abiDef abi.ABI) (string, error) {
	event, exists := abiDef.Events[EventNameTeeInstructionsSent]
	if !exists {
		return "", fmt.Errorf("event %s not found", EventNameTeeInstructionsSent)
	}
	return event.ID.Hex(), nil
}

func DecodeTeeInstructionsSentEventData(log *types.Log, teeAbi abi.ABI, paymentAbi abi.ABI) (*payment.ITeePaymentsPaymentInstructionMessage, error) {
	eventData, err := utils.AbiDecodeEventData[teeextensionregistry.TeeExtensionRegistryTeeInstructionsSent](
		teeAbi,
		EventNameTeeInstructionsSent,
		log.Data,
	)
	if err != nil {
		return nil, err
	}
	var message payment.ITeePaymentsPaymentInstructionMessage
	err = paymentAbi.UnpackIntoInterface(&message, PaymentInstructionMessage, eventData.Message)
	if err != nil {
		return nil, err
	}
	return &message, nil
}
