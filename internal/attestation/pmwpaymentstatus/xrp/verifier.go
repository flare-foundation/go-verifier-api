package xrpverifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwnonce"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/builder"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type XRPVerifier struct {
	Repo   *db.DBRepo
	Config *config.PMWPaymentStatusConfig
	// Binder binds a payment's XRP sequence to its paymentId via on-chain
	// initialNonce. An interface so tests can substitute a stub.
	Binder pmwnonce.SequenceVerifier
}

func NewXRPVerifier(cfg *config.PMWPaymentStatusConfig, xrpDB, cChainDB *gorm.DB) (*XRPVerifier, error) {
	binder, err := pmwnonce.NewOnChainBinder(cfg.RPCURL, cfg.FlareTeeManagerContractAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot create initial-nonce binder: %w", err)
	}
	return &XRPVerifier{
		Repo:   db.NewDBRepo(xrpDB, cChainDB, cfg.FlareTeeManagerContractAddress),
		Config: cfg,
		Binder: binder,
	}, nil
}

// Close releases the binder's RPC connection. Implements io.Closer so the
// service can release it at shutdown alongside the DB connections.
func (x *XRPVerifier) Close() error {
	if c, ok := x.Binder.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func (x *XRPVerifier) Verify(ctx context.Context, req fdc2.IPMWPaymentStatusRequestBody) (fdc2.IPMWPaymentStatusResponseBody, error) {
	instructionID, err := teeinstruction.GenerateInstructionID(req.OpType, x.Config.SourceIDPair.SourceIDEncoded, req.SenderAddress, req.PaymentId)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot generate instruction ID: %w", err)
	}
	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(x.Config.ParsedTeeInstructionsABI)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	chainLog, err := x.Repo.FetchInstructionLog(ctx, eventHash, instructionID)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	paymentMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(chainLog, x.Config.ParsedTeeInstructionsABI, op.Pay, req.OpType)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	// Bind the decoded event back to the request. The instruction-ID topic already
	// commits to these fields, so a mismatch means the indexed event data disagrees
	// with its own topic — a C-chain index inconsistency (counterpart to the XRP row
	// consistency check below).
	if err := db.CheckInstructionConsistency(paymentMessage, x.Config.SourceIDPair.SourceIDEncoded, req.SenderAddress, req.PaymentId); err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	// Bind the event's XRP sequence to the value the contract deterministically
	// assigns this paymentId (initialNonce + paymentId - 1), read on-chain. The
	// instruction-ID topic does not commit to the sequence, so without this a
	// compromised indexer could point a paymentId at a foreign sequence.
	if err := x.Binder.VerifySequence(ctx, x.Config.SourceIDPair.SourceIDEncoded, req.SenderAddress, req.PaymentId, paymentMessage.Nonce); err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	// The request no longer carries the XRP sequence; it comes from the decoded
	// event message (paymentMessage.Nonce). The row is bound to that sequence by
	// CheckRowConsistency below.
	dbTransaction, err := x.Repo.FetchTransactionBySourceAndSequence(ctx, db.ChainQuery{SourceAddress: req.SenderAddress, Nonce: paymentMessage.Nonce})
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	rawTransactionData, err := x.parseRawTransactionData(req.SenderAddress, paymentMessage.Nonce, dbTransaction.Response)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	// Cross-check: JSON payload must describe the same XRPL transaction as the canonical DB columns.
	// A row where they disagree is evidence of indexer corruption or partial write — refuse the attestation.
	if err := db.CheckRowConsistency(rawTransactionData.Hash, rawTransactionData.Account, uint64(rawTransactionData.Sequence), dbTransaction); err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	// Validate transaction and build response
	resp, err := builder.BuildPaymentStatusResponse(rawTransactionData, paymentMessage, dbTransaction)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot build payment status response: %w", err)
	}
	return resp, nil
}

const maxResponseSize = 1 << 20

func (x *XRPVerifier) parseRawTransactionData(sender string, nonce uint64, response string) (types.RawTransactionData, error) {
	var rawTransactionData types.RawTransactionData
	if len(response) > maxResponseSize {
		return rawTransactionData, fmt.Errorf("XRP transaction response too large: %d bytes (max %d): %w", len(response), maxResponseSize, db.ErrDatabase)
	}
	err := json.Unmarshal([]byte(response), &rawTransactionData)
	if err != nil {
		logger.Errorf("Cannot unmarshal XRP transaction response for %s with nonce %d: %v", sender, nonce, err)
		return rawTransactionData, fmt.Errorf("cannot unmarshal XRP transaction response: %w", err)
	}
	if rawTransactionData.MetaData.TransactionResult == "" {
		return rawTransactionData, errors.New("missing transaction result in raw transaction data")
	}
	return rawTransactionData, nil
}
