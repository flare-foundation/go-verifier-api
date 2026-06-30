package builder

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/helper"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/transaction"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/types"
)

func BuildPaymentStatusResponse(
	raw types.RawTransactionData,
	paymentMsg *payments.ITeePaymentsPaymentInstructionMessage,
	tx db.DBTransaction,
) (fdc2.IPMWPaymentStatusResponseBody, error) {
	var zero fdc2.IPMWPaymentStatusResponseBody
	if raw.TransactionType != "Payment" {
		return zero, fmt.Errorf("expected Payment transaction, got %q", raw.TransactionType)
	}
	if len(paymentMsg.TokenId) != 0 {
		return zero, errors.New("non-native payments (TokenId set) are not supported")
	}
	// Fail closed on physically-impossible drops magnitudes. No legitimate XRPL
	// amount/fee can exceed the total XRP supply, so a larger value is corrupt or
	// forged data — bound the instruction-derived Amount/MaxFee and the
	// indexer-derived fee/received-amount below before they reach the response.
	if err := checkDropsBound("payment amount", paymentMsg.Amount); err != nil {
		return zero, err
	}
	if err := checkDropsBound("payment maxFee", paymentMsg.MaxFee); err != nil {
		return zero, err
	}
	// The transaction outcome below (status, revert reason, received amount, fee) is
	// read from the indexer's transaction JSON. CheckRowConsistency has bound this
	// row's identity (hash/account/sequence) to the request, but the outcome content
	// is not independently verifiable against the XRPL ledger — the indexer is
	// semi-trusted for it (mitigated by FDC multi-verifier consensus; see the
	// "Indexer payload is semi-trusted" accepted risk in docs/SPEC.md). The checks
	// below only fail closed on physically-impossible / malformed values.
	transactionResult, err := getTransactionStatus(raw.MetaData.TransactionResult)
	if err != nil {
		return zero, fmt.Errorf("cannot parse transaction status: %w", err)
	}
	transactionFee, err := helper.ParseNonNegativeBigInt(raw.Fee)
	if err != nil {
		return zero, fmt.Errorf("invalid transaction fee %q: %w", raw.Fee, err)
	}
	if err := checkDropsBound("transaction fee", transactionFee); err != nil {
		return zero, err
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
	if err := checkDropsBound("received amount", receivedAmount); err != nil {
		return zero, err
	}
	revertReason := ""
	if transactionResult != types.Success {
		revertReason = raw.MetaData.TransactionResult
	}
	return fdc2.IPMWPaymentStatusResponseBody{
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

// getTransactionStatus maps an XRPL transaction result code to a status. Only the
// classes that can appear in a *validated ledger* are accepted: `tes*` (success)
// and `tec*` (failed, but the fee was claimed and the transaction is included in
// the ledger). The local/pre-consensus classes `tef`/`tel`/`tem`/`ter` never reach
// a validated ledger, so an indexer (which serves validated-ledger data) presenting
// one — or any unrecognized code — is corrupt or forged data and fails closed
// rather than being attested as a real on-ledger payment outcome.
// https://xrpl.org/docs/references/protocol/transactions/transaction-results
func getTransactionStatus(result string) (types.TransactionStatus, error) {
	const transactionResultPrefixLength = 3
	if len(result) < transactionResultPrefixLength {
		return 0, fmt.Errorf("transaction result too short: %q: %w", result, db.ErrDatabase)
	}
	switch result[:transactionResultPrefixLength] {
	case "tes":
		return types.Success, nil
	case "tec":
		return types.Reverted, nil
	default:
		return 0, fmt.Errorf("non-validated or unrecognized XRPL transaction result code %q: %w", result, db.ErrDatabase)
	}
}

// checkDropsBound fails closed when an XRP drops amount exceeds the total XRP
// supply (helper.MaxXRPDrops) — physically impossible, hence corrupt/forged data.
// A nil value is likewise rejected rather than panicking the bound check.
func checkDropsBound(name string, v *big.Int) error {
	if v == nil {
		return fmt.Errorf("%s is nil: %w", name, db.ErrDatabase)
	}
	if helper.ExceedsMaxXRPDrops(v) {
		return fmt.Errorf("%s %s exceeds total XRP supply in drops (%d): %w", name, v, helper.MaxXRPDrops, db.ErrDatabase)
	}
	return nil
}
