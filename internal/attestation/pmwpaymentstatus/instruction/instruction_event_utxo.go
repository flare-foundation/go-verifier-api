package instruction

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
)

// utxoMessageStructMethod is the payments ABI method whose sole input is the
// Bitcoin UtxoPaymentInstructionMessage tuple.
const utxoMessageStructMethod = "utxoPaymentInstructionMessageStruct"

// utxoMessageArgument is the ABI argument for the UtxoPaymentInstructionMessage
// tuple. The payments package's MessageArguments map wires op.Pay to the XRP
// paymentInstructionMessageStruct, so the Bitcoin message argument is resolved
// separately from its ABI method here.
var utxoMessageArgument abi.Argument

func init() {
	paymentsABI, err := payments.TeePaymentsMetaData.GetAbi()
	if err != nil {
		panic(fmt.Sprintf("cannot load payments ABI: %v", err))
	}
	method, ok := paymentsABI.Methods[utxoMessageStructMethod]
	if !ok {
		panic("payments ABI missing method " + utxoMessageStructMethod)
	}
	utxoMessageArgument = method.Inputs[0]
}

// DecodeUtxoTeeInstructionsSentEventData decodes a TeeInstructionsSent event
// whose message is a Bitcoin UtxoPaymentInstructionMessage. It binds the wrapper
// op fields (OpType, OpCommand) exactly as DecodeTeeInstructionsSentEventData
// does for the XRP message, then decodes the payload into the UTXO schema.
func DecodeUtxoTeeInstructionsSentEventData(log *types.Log, teeABI abi.ABI, command op.Command, expectedOpType common.Hash) (*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, error) {
	messageData, err := decodeInstructionEnvelope(log, teeABI, command, expectedOpType)
	if err != nil {
		return nil, err
	}
	var message payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage
	if err := structs.DecodeTo(utxoMessageArgument, messageData, &message); err != nil {
		return nil, fmt.Errorf("cannot decode %s UTXO message arguments: %w", EventNameTeeInstructionsSent, err)
	}
	return &message, nil
}

// DecodeUtxoPaymentInstructionMessage decodes a bare ABI-encoded
// UtxoPaymentInstructionMessage.
//
// The sibling above unwraps a TeeInstructionsSent envelope first, because the
// diamond carries the message inside an instruction. The CSP channel carries the
// same struct on its own, in a PaymentBatched record that is not an instruction,
// so only this half applies there.
func DecodeUtxoPaymentInstructionMessage(messageData []byte) (*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, error) {
	var message payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage
	if err := structs.DecodeTo(utxoMessageArgument, messageData, &message); err != nil {
		return nil, fmt.Errorf("cannot decode UTXO payment instruction message: %w", err)
	}
	return &message, nil
}
