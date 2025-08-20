package builder

import (
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/models"
	xrptypes "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/types"
	pmwpaymentutils "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/utils"
	xrputils "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp_utils"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

func BuildPaymentStatusResponse(
	raw xrptypes.RawTransactionData,
	paymentMsg *payment.ITeePaymentsPaymentInstructionMessage,
	tx models.DBTransaction,
) (connector.IPMWPaymentStatusResponseBody, error) {

	transactionResult, err := xrputils.GetTransactionStatus(raw.MetaData.TransactionResult)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	transactionFee, err := utils.NewBigIntFromString(raw.Fee)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	hashBytes, err := utils.HexStringToBytes32(tx.Hash)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	receivedAmount, err := xrputils.FindReceivedAmountForAddress(&raw.MetaData, paymentMsg.RecipientAddress)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	revertReason := ""
	if transactionResult != xrptypes.Success {
		revertReason = raw.MetaData.TransactionResult
	}
	return connector.IPMWPaymentStatusResponseBody{
		SenderAddress:     pmwpaymentutils.GetStandardAddressHash(paymentMsg.SenderAddress),
		RecipientAddress:  pmwpaymentutils.GetStandardAddressHash(paymentMsg.RecipientAddress),
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
