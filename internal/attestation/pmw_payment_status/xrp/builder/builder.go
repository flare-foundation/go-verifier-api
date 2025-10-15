package builder

import (
	"fmt"

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
	var zero connector.IPMWPaymentStatusResponseBody
	transactionResult, err := getTransactionStatus(raw.MetaData.TransactionResult)
	if err != nil {
		return zero, fmt.Errorf("cannot parse transaction status: %w", err)
	}
	transactionFee, err := helper.ParseBigInt(raw.Fee)
	if err != nil {
		return zero, fmt.Errorf("invalid transaction fee %q: %w", raw.Fee, err)
	}
	hashBytes, err := utils.HexStringToBytes32(tx.Hash)
	if err != nil {
		return zero, fmt.Errorf("invalid transaction hash %s: %w", tx.Hash, err)
	}
	receivedAmount, err := transaction.FindReceivedAmountForAddress(&raw.MetaData, paymentMsg.RecipientAddress)
	if err != nil {
		return zero, fmt.Errorf("cannot calculate received amount for recipient %s: %w", paymentMsg.RecipientAddress, err)
	}
	revertReason := ""
	if transactionResult != types.Success {
		revertReason = raw.MetaData.TransactionResult
	}
	return connector.IPMWPaymentStatusResponseBody{
		RecipientAddress:  paymentMsg.RecipientAddress,
		TokenId:           paymentMsg.TokenId,
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

// https://xrpl.org/docs/references/protocol/transactions/transaction-results
func getTransactionStatus(result string) (types.TransactionStatus, error) {
	const transactionResultPrefixLength = 3
	if len(result) < transactionResultPrefixLength {
		return 0, fmt.Errorf("transaction result too short: %q", result) // Should never happen.
	}
	prefix := result[:transactionResultPrefixLength]
	if prefix == "tes" {
		return types.Success, nil
	}
	return types.Reverted, nil
}
