package xrpverifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
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
		Repo:   db.NewDBRepo(xrpDB, cChainDB),
		Config: cfg,
	}
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWPaymentStatusRequestBody) (connector.IPMWPaymentStatusResponseBody, error) {
	// Build instruction ID
	instructionID, err := teeinstruction.GenerateInstructionID(req.OpType, x.Config.SourceIDPair.SourceIDEncoded, req.SenderAddress, req.Nonce)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot generate instruction ID: %w", err)
	}
	// Event log
	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(x.Config.ParsedTeeInstructionsABI)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	chainLog, err := x.Repo.FetchInstructionLog(ctx, eventHash, instructionID)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Decode event data
	paymentMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(chainLog, x.Config.ParsedTeeInstructionsABI, op.Pay)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Query underlying chain for transaction
	dbTransaction, err := x.Repo.FetchTransactionBySourceAndSequence(ctx, db.ChainQuery{SourceAddress: req.SenderAddress, Nonce: req.Nonce})
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Parse transaction response JSON into structured data
	rawTransactionData, err := x.parseRawTransactionData(req.SenderAddress, req.Nonce, dbTransaction.Response)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Validate transaction and build response
	resp, err := builder.BuildPaymentStatusResponse(rawTransactionData, paymentMessage, dbTransaction)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot build payment status response: %w", err)
	}
	return resp, nil
}

func (x *XRPVerifier) parseRawTransactionData(sender string, nonce uint64, response string) (types.RawTransactionData, error) {
	var rawTransactionData types.RawTransactionData
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
