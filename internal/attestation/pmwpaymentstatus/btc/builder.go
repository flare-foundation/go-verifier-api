package btcverifier

import (
	"encoding/hex"
	"fmt"
	"math/big"

	btcbatch "github.com/flare-foundation/go-flare-common/pkg/btc/batch"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc/batchtx"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
)

// revertReasonSubDust is the revertReason for an undeliverable (K=0) payment:
// the instruction amount fell below the recipient's dust threshold and was
// redirected to change (spec PMWPaymentStatus-BTC.md, "undeliverable handling").
const revertReasonSubDust = "sub_dust_amount"

// txidHexLen is the hex length of a 32-byte Bitcoin txid.
const txidHexLen = 64

// baseResponse fills the instruction-derived fields shared by every response,
// regardless of settlement status. Monetary fields are never nil so the ABI
// encoder cannot panic on a missing big.Int.
func baseResponse(m *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) fdc2.IPMWPaymentStatusResponseBody {
	tokenID := m.TokenId
	if tokenID == nil {
		tokenID = []byte{}
	}
	return fdc2.IPMWPaymentStatusResponseBody{
		RecipientAddress: m.RecipientAddress,
		TokenId:          tokenID,
		Amount:           bigOrZero(m.Amount),
		MaxFee:           bigOrZero(m.MaxFee),
		PaymentReference: m.PaymentReference,
		ReceivedAmount:   big.NewInt(0),
		// Zero for not-found / undeliverable; buildResponse fills the real
		// whole-batch fee for a settled payment.
		TransactionFee: big.NewInt(0),
	}
}

// notFoundResponse is returned when no confirmed batch transaction settles the
// payment (transactionStatus = 2). The instruction fields are still populated.
func notFoundResponse(m *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) fdc2.IPMWPaymentStatusResponseBody {
	resp := baseResponse(m)
	resp.TransactionStatus = uint8(btcbatch.StatusNotFound)
	return resp
}

// buildResponse assembles the response for a payment found in a confirmed batch
// transaction. status is btcbatch.Match's verdict (success or undeliverable);
// received is the satoshis paid to the recipient. transactionFee and
// blockTimestamp come from the indexer's batch row.
func buildResponse(m *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, status btcbatch.Status, received int64, batch *batchtx.BatchTx) (fdc2.IPMWPaymentStatusResponseBody, error) {
	txidBytes, err := txidToBytes32(batch.Txid)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, fmt.Errorf("node returned invalid txid %q: %w: %w", batch.Txid, err, paymentdb.ErrDatabase)
	}
	resp := baseResponse(m)
	resp.TransactionStatus = uint8(status)
	resp.TransactionId = txidBytes
	resp.BlockNumber = batch.BlockNumber
	resp.BlockTimestamp = batch.BlockTimestamp
	resp.TransactionFee = big.NewInt(batch.Fee)
	switch status {
	case btcbatch.StatusSuccess:
		resp.ReceivedAmount = big.NewInt(received)
	case btcbatch.StatusUndeliverable:
		resp.RevertReason = revertReasonSubDust
	case btcbatch.StatusNotFound:
		// Not reached here: a not-found settlement is built by notFoundResponse.
		// Enumerated for exhaustiveness.
	}
	return resp, nil
}

// bigOrZero returns v, or a zero big.Int when v is nil.
func bigOrZero(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	return v
}

// txidToBytes32 decodes a 64-hex-character Bitcoin txid into a 32-byte array in
// the same (display) byte order, so hex-encoding the response field reproduces
// the txid string.
func txidToBytes32(txid string) ([32]byte, error) {
	var out [32]byte
	if len(txid) != txidHexLen {
		return out, fmt.Errorf("txid must be %d hex chars, got %d", txidHexLen, len(txid))
	}
	raw, err := hex.DecodeString(txid)
	if err != nil {
		return out, fmt.Errorf("txid is not valid hex: %w", err)
	}
	copy(out[:], raw)
	return out, nil
}
