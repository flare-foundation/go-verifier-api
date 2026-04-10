package builder

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/helper"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/transaction"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/types"
)

func BuildPaymentStatusResponse(
	raw types.RawTransactionData,
	paymentMsg *payment.ITeePaymentsPaymentInstructionMessage,
	tx db.DBTransaction,
) (connector.IPMWPaymentStatusResponseBody, error) {
	var zero connector.IPMWPaymentStatusResponseBody
	if raw.TransactionType != "Payment" {
		return zero, fmt.Errorf("expected Payment transaction, got %q", raw.TransactionType)
	}
	transactionResult, err := getTransactionStatus(raw.MetaData.TransactionResult)
	if err != nil {
		return zero, fmt.Errorf("cannot parse transaction status: %w", err)
	}
	transactionFee, err := helper.ParseBigInt(raw.Fee)
	if err != nil {
		return zero, fmt.Errorf("invalid transaction fee %q: %w", raw.Fee, err)
	}
	hashBytes, err := convert.Hex32StringToCommonHash(tx.Hash)
	if err != nil {
		return zero, fmt.Errorf("invalid transaction hash %s: %w", tx.Hash, err)
	}
	// Normalize recipient address: X-addresses (used by some XRPL clients) are decoded
	// to classic r... addresses for matching against transaction metadata, which always
	// uses classic addresses.
	recipientClassic, err := helper.NormalizeAddress(paymentMsg.RecipientAddress)
	if err != nil {
		return zero, fmt.Errorf("invalid recipient address %s: %w", paymentMsg.RecipientAddress, err)
	}
	// NOTE: receivedAmount is calculated from AffectedNodes regardless of transaction status.
	// For reverted XRP transactions (tec-class results), this is typically 0 since the recipient's
	// balance is unchanged. We intentionally calculate rather than hardcode 0 on revert, because
	// it reports what actually happened on-chain and would self-correct if an edge case ever
	// modifies the recipient's balance on a non-tesSUCCESS result.
	receivedAmount, err := transaction.FindReceivedAmountForAddress(&raw.MetaData, recipientClassic)
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
		MaxFee:            paymentMsg.MaxFee,
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
		return 0, fmt.Errorf("transaction result too short: %q", result)
	}
	prefix := result[:transactionResultPrefixLength]
	if prefix == "tes" {
		return types.Success, nil
	}
	return types.Reverted, nil
}
