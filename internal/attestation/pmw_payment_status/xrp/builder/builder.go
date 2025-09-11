package builder

import (
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	utils "github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/helper"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/model"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/transaction"
	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
)

func BuildPaymentStatusResponse(
	raw types.RawTransactionData,
	paymentMsg *payment.ITeePaymentsPaymentInstructionMessage,
	tx model.DBTransaction,
) (connector.IPMWPaymentStatusResponseBody, error) {
	transactionResult, err := transaction.GetTransactionStatus(raw.MetaData.TransactionResult)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	transactionFee, err := helper.ParseBigInt(raw.Fee)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	hashBytes, err := utils.HexStringToBytes32(tx.Hash)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	receivedAmount, err := transaction.FindReceivedAmountForAddress(&raw.MetaData, paymentMsg.RecipientAddress)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	revertReason := ""
	if transactionResult != types.Success {
		revertReason = raw.MetaData.TransactionResult
	}
	return connector.IPMWPaymentStatusResponseBody{
		SenderAddress:     helper.GetStandardAddressHash(paymentMsg.SenderAddress),
		RecipientAddress:  helper.GetStandardAddressHash(paymentMsg.RecipientAddress),
		Amount:            paymentMsg.Amount,
		Fee:               paymentMsg.Fee,
		PaymentReference:  paymentMsg.PaymentReference,
		TransactionStatus: uint8(transactionResult),
		RevertReason:      revertReason,
		ReceivedAmount:    receivedAmount,
		TransactionFee:    transactionFee,
		TransactionId:     hashBytes,
		BlockNumber:       tx.BlockNumber,
		BlockTimestamp:    tx.Timestamp,
	}, nil
}
