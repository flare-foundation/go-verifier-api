package verification

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeinstructions"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
)

const eventNameTeeInstructionsSent = "TeeInstructionsSent"

var (
	parsedTeeInstructionsABI abi.ABI
	parsedPaymentABI         abi.ABI
	initErr                  error
)

func init() {
	parsedTeeInstructionsABI, initErr = abi.JSON(strings.NewReader(teeinstructions.TeeInstructionsABI))
	if initErr != nil {
		panic(initErr)
	}
	parsedPaymentABI, initErr = abi.JSON(strings.NewReader(payment.PaymentMetaData.ABI))
	if initErr != nil {
		panic(initErr)
	}
}

func GetTeeInstructionsSentEventSignature() (string, error) {
	if initErr != nil {
		return "", initErr
	}
	event, exists := parsedTeeInstructionsABI.Events[eventNameTeeInstructionsSent]
	if !exists {
		return "", fmt.Errorf("event %s not found", eventNameTeeInstructionsSent)
	}
	eventSignature := event.ID.Hex()
	return eventSignature, nil
}

func DecodeTeeInstructionsSentEventData(log *types.Log) (*payment.ITeePaymentsPaymentInstructionMessage, error) {
	if initErr != nil {
		return nil, initErr
	}
	var eventData teeinstructions.TeeInstructionsTeeInstructionsSent
	err := parsedTeeInstructionsABI.UnpackIntoInterface(&eventData, eventNameTeeInstructionsSent, log.Data)
	if err != nil {
		return nil, err
	}
	var message payment.ITeePaymentsPaymentInstructionMessage
	err = parsedPaymentABI.UnpackIntoInterface(&message, "paymentInstructionMessageStruct", eventData.Message)
	if err != nil {
		return nil, err
	}
	return &message, nil
}
