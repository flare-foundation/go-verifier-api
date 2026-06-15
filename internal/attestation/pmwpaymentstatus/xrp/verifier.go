package xrpverifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
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
}

func NewXRPVerifier(cfg *config.PMWPaymentStatusConfig, xrpDB, cChainDB *gorm.DB) *XRPVerifier {
	return &XRPVerifier{
		Repo:   db.NewDBRepo(xrpDB, cChainDB, cfg.FlareTeeManagerContractAddress),
		Config: cfg,
	}
}

func (x *XRPVerifier) Verify(ctx context.Context, req fdc2.IPMWPaymentStatusRequestBody) (fdc2.IPMWPaymentStatusResponseBody, error) {
	instructionID, err := teeinstruction.GenerateInstructionID(req.OpType, x.Config.SourceIDPair.SourceIDEncoded, req.SenderAddress, req.Nonce)
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
	paymentMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(chainLog, x.Config.ParsedTeeInstructionsABI, op.Pay)
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	dbTransaction, err := x.Repo.FetchTransactionBySourceAndSequence(ctx, db.ChainQuery{SourceAddress: req.SenderAddress, Nonce: req.Nonce})
	if err != nil {
		return fdc2.IPMWPaymentStatusResponseBody{}, err
	}
	rawTransactionData, err := x.parseRawTransactionData(req.SenderAddress, req.Nonce, dbTransaction.Response)
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
