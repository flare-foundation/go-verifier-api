package builder_test

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-flare-common/pkg/xrpl/transactions"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/builder"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/types"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	"github.com/stretchr/testify/require"
)

func TestBuildPaymentStatusResponse(t *testing.T) {
	paymentMessageInstruction := payment.ITeePaymentsPaymentInstructionMessage{
		RecipientAddress: "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
		TokenId:          []byte{0},
		Amount:           big.NewInt(10000000),
		MaxFee:           big.NewInt(12),
		PaymentReference: [32]byte{0},
	}
	rawTransactionData := types.RawTransactionData{
		CommonFields: transactions.CommonFields{
			Account:         "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
			TransactionType: "Payment",
			Fee:             "12",
			Sequence:        uint(0),
			Memos: []transactions.Memo{
				{
					MemoData:   "",
					MemoFormat: "",
					MemoType:   "",
				},
			},
		},
		MetaData: helpers.PaymentTransaction0.MetaData,
	}
	txFromDB := db.DBTransaction{
		Hash:        "4818566F359119B16544087CEA17CE2E7152A5BD4B21572C809A9AA5A7DE2B2F",
		BlockNumber: uint64(10110065),
		Timestamp:   uint64(1756296242),
	}
	t.Run("success", func(t *testing.T) {
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &paymentMessageInstruction, txFromDB)
		require.NoError(t, err)
		require.Equal(t, paymentMessageInstruction.Amount, val.Amount)
		require.Equal(t, txFromDB.BlockNumber, val.BlockNumber)
		require.Equal(t, txFromDB.Timestamp, val.BlockTimestamp)
		require.Equal(t, paymentMessageInstruction.TokenId, val.TokenId)
		require.Equal(t, paymentMessageInstruction.RecipientAddress, val.RecipientAddress)
		require.Equal(t, paymentMessageInstruction.PaymentReference, val.PaymentReference)
		require.Equal(t, "", val.RevertReason)
		require.Equal(t, paymentMessageInstruction.Amount, val.ReceivedAmount)
		require.Equal(t, paymentMessageInstruction.MaxFee, val.TransactionFee)
		require.Equal(t, uint8(types.Success), val.TransactionStatus)
		require.Equal(t, strings.ToLower(txFromDB.Hash), hex.EncodeToString(val.TransactionId[:]))
	})
	t.Run("success - different status", func(t *testing.T) {
		modRawTransactionData := rawTransactionData
		modRawTransactionData.MetaData.TransactionResult = "tecNO_DST_INSUF_XRP"
		val, err := builder.BuildPaymentStatusResponse(modRawTransactionData, &paymentMessageInstruction, txFromDB)
		require.NoError(t, err)
		require.Equal(t, paymentMessageInstruction.Amount, val.Amount)
		require.Equal(t, txFromDB.BlockNumber, val.BlockNumber)
		require.Equal(t, txFromDB.Timestamp, val.BlockTimestamp)
		require.Equal(t, paymentMessageInstruction.TokenId, val.TokenId)
		require.Equal(t, paymentMessageInstruction.RecipientAddress, val.RecipientAddress)
		require.Equal(t, paymentMessageInstruction.PaymentReference, val.PaymentReference)
		require.Equal(t, "tecNO_DST_INSUF_XRP", val.RevertReason)
		require.Equal(t, paymentMessageInstruction.Amount, val.ReceivedAmount)
		require.Equal(t, paymentMessageInstruction.MaxFee, val.TransactionFee)
		require.Equal(t, uint8(types.Reverted), val.TransactionStatus)
		require.Equal(t, strings.ToLower(txFromDB.Hash), hex.EncodeToString(val.TransactionId[:]))
	})
	t.Run("invalid transaction status", func(t *testing.T) {
		modRawTransactionData := rawTransactionData
		modRawTransactionData.MetaData.TransactionResult = "te"
		val, err := builder.BuildPaymentStatusResponse(modRawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, connector.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "transaction result too short")
	})
	t.Run("invalid fee field", func(t *testing.T) {
		modRawTransactionData := rawTransactionData
		modRawTransactionData.Fee = "fee"
		val, err := builder.BuildPaymentStatusResponse(modRawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, connector.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "invalid big.Int string: fee")
	})
	t.Run("invalid tx hash field", func(t *testing.T) {
		txFromDB := db.DBTransaction{
			Hash: "0x1234",
		}
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, connector.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "invalid transaction hash 0x1234: invalid length for hex string 0x1234: expected 32 bytes, got 2")
	})
	t.Run("no meta data", func(t *testing.T) {
		val, err := builder.BuildPaymentStatusResponse(helpers.PaymentTransaction0_error0, &paymentMessageInstruction, txFromDB)
		require.Equal(t, connector.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "cannot calculate received amount for recipient")
		require.ErrorContains(t, err, "invalid balance format in CreatedNode for account")
	})
}
