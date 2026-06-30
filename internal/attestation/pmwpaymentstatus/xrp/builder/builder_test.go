package builder_test

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"testing"

	addresscodec "github.com/Peersyst/xrpl-go/address-codec"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/flare-foundation/go-flare-common/pkg/xrpl/transactions"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/helper"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/builder"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/types"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	"github.com/stretchr/testify/require"
)

func TestBuildPaymentStatusResponse(t *testing.T) {
	paymentMessageInstruction := payments.ITeePaymentsPaymentInstructionMessage{
		RecipientAddress: "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
		TokenId:          []byte{},
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
				{},
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
	t.Run("success with X-address recipient", func(t *testing.T) {
		// Convert the classic recipient to an X-address to verify normalization in the builder.
		xAddr, err := addresscodec.ClassicAddressToXAddress("rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK", 0, false, false)
		require.NoError(t, err)

		xAddrInstruction := payments.ITeePaymentsPaymentInstructionMessage{
			RecipientAddress: xAddr,
			TokenId:          []byte{},
			Amount:           big.NewInt(10000000),
			MaxFee:           big.NewInt(12),
			PaymentReference: [32]byte{0},
		}
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &xAddrInstruction, txFromDB)
		require.NoError(t, err)
		// RecipientAddress in response should be the original X-address from the instruction.
		require.Equal(t, xAddr, val.RecipientAddress)
		// ReceivedAmount must match — the normalization converts X-address to classic for metadata lookup.
		require.Equal(t, paymentMessageInstruction.Amount, val.ReceivedAmount)
		require.Equal(t, uint8(types.Success), val.TransactionStatus)
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
	t.Run("non-native payment rejected", func(t *testing.T) {
		iouInstruction := payments.ITeePaymentsPaymentInstructionMessage{
			RecipientAddress: "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
			TokenId:          []byte{0x01},
			Amount:           big.NewInt(10000000),
			MaxFee:           big.NewInt(12),
		}
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &iouInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "non-native payments (TokenId set) are not supported")
	})
	t.Run("non-payment transaction type rejected", func(t *testing.T) {
		modRawTransactionData := rawTransactionData
		modRawTransactionData.TransactionType = "AccountSet"
		val, err := builder.BuildPaymentStatusResponse(modRawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, `expected Payment transaction, got "AccountSet"`)
	})
	t.Run("invalid transaction status", func(t *testing.T) {
		modRawTransactionData := rawTransactionData
		modRawTransactionData.MetaData.TransactionResult = "te"
		val, err := builder.BuildPaymentStatusResponse(modRawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "transaction result too short")
	})
	t.Run("invalid fee field", func(t *testing.T) {
		modRawTransactionData := rawTransactionData
		modRawTransactionData.Fee = "fee"
		val, err := builder.BuildPaymentStatusResponse(modRawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "invalid big.Int string: fee")
	})
	t.Run("negative fee field rejected", func(t *testing.T) {
		// Corrupted or malicious indexer data with a negative Fee must not
		// propagate into the FDC2 response (XRPL fees are unsigned drops).
		modRawTransactionData := rawTransactionData
		modRawTransactionData.Fee = "-1"
		val, err := builder.BuildPaymentStatusResponse(modRawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "invalid transaction fee")
		require.ErrorContains(t, err, "expected non-negative big.Int")
	})
	t.Run("invalid tx hash field", func(t *testing.T) {
		txFromDB := db.DBTransaction{
			Hash: "0x1234",
		}
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "invalid transaction hash 0x1234: invalid length for hex string 0x1234: expected 32 bytes, got 2")
	})
	t.Run("no meta data", func(t *testing.T) {
		val, err := builder.BuildPaymentStatusResponse(helpers.PaymentTransaction0_error0, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorContains(t, err, "cannot calculate received amount for recipient")
		require.ErrorContains(t, err, "invalid balance format in CreatedNode for account")
	})
	t.Run("transaction fee above XRP supply fails closed", func(t *testing.T) {
		modRaw := rawTransactionData
		modRaw.Fee = strconv.FormatUint(helper.MaxXRPDrops+1, 10)
		val, err := builder.BuildPaymentStatusResponse(modRaw, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "transaction fee")
	})
	t.Run("received amount above XRP supply fails closed", func(t *testing.T) {
		// Craft metadata where the recipient's AccountRoot balance jumps by more than
		// the total XRP supply — a physically-impossible delta from a corrupt indexer.
		huge := strconv.FormatUint(helper.MaxXRPDrops+1, 10)
		modRaw := rawTransactionData
		modRaw.MetaData = types.TransactionMetaData{
			TransactionResult: "tesSUCCESS",
			AffectedNodes: []types.AffectedNode{
				{ModifiedNode: &types.ModifiedNode{
					LedgerEntryType: "AccountRoot",
					FinalFields:     map[string]any{"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK", "Balance": huge},
					PreviousFields:  map[string]any{"Balance": "0"},
				}},
			},
		}
		val, err := builder.BuildPaymentStatusResponse(modRaw, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "received amount")
	})
	t.Run("garbage transaction result code rejected", func(t *testing.T) {
		modRaw := rawTransactionData
		modRaw.MetaData.TransactionResult = "xyzGARBAGE"
		val, err := builder.BuildPaymentStatusResponse(modRaw, &paymentMessageInstruction, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "unrecognized XRPL transaction result code")
	})
	t.Run("result code classes (validated-ledger only)", func(t *testing.T) {
		cases := []struct {
			result string
			status types.TransactionStatus
			reject bool // local/pre-consensus classes must fail closed
		}{
			{"tesSUCCESS", types.Success, false},
			{"tecNO_DST_INSUF_XRP", types.Reverted, false},
			{"tefPAST_SEQ", 0, true},
			{"telLOCAL_ERROR", 0, true},
			{"temMALFORMED", 0, true},
			{"terPRE_SEQ", 0, true},
		}
		for _, c := range cases {
			t.Run(c.result, func(t *testing.T) {
				modRaw := rawTransactionData
				modRaw.MetaData.TransactionResult = c.result
				val, err := builder.BuildPaymentStatusResponse(modRaw, &paymentMessageInstruction, txFromDB)
				if c.reject {
					require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
					require.ErrorIs(t, err, db.ErrDatabase)
					return
				}
				require.NoError(t, err)
				require.Equal(t, uint8(c.status), val.TransactionStatus)
			})
		}
	})
	t.Run("instruction amount above XRP supply fails closed", func(t *testing.T) {
		msg := paymentMessageInstruction
		msg.Amount = new(big.Int).SetUint64(helper.MaxXRPDrops + 1)
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &msg, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "payment amount")
	})
	t.Run("instruction maxFee above XRP supply fails closed", func(t *testing.T) {
		msg := paymentMessageInstruction
		msg.MaxFee = new(big.Int).SetUint64(helper.MaxXRPDrops + 1)
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &msg, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "payment maxFee")
	})
	t.Run("nil instruction amount rejected", func(t *testing.T) {
		msg := paymentMessageInstruction
		msg.Amount = nil
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &msg, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "payment amount is nil")
	})
	t.Run("nil instruction maxFee rejected", func(t *testing.T) {
		msg := paymentMessageInstruction
		msg.MaxFee = nil
		val, err := builder.BuildPaymentStatusResponse(rawTransactionData, &msg, txFromDB)
		require.Equal(t, fdc2.IPMWPaymentStatusResponseBody{}, val)
		require.ErrorIs(t, err, db.ErrDatabase)
		require.ErrorContains(t, err, "payment maxFee is nil")
	})
}
