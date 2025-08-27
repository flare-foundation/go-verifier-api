package verifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/contracts/teewalletmanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teewalletprojectmanager"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/builder"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/decoder"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/repo"
	xrptypes "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/types"
	pmwpaymentutils "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/utils"
	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type XRPVerifier struct {
	repo                 *repo.XRPRepository
	WalletManagerCaller  *teewalletmanager.TeeWalletManagerCaller
	ProjectManagerCaller *teewalletprojectmanager.TeeWalletProjectManagerCaller
	config               *pmwpaymentstatusconfig.PMWPaymentStatusConfig
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWPaymentStatusRequestBody) (connector.IPMWPaymentStatusResponseBody, error) {
	// Build instruction Id
	opType, err := pmwpaymentutils.GetWalletOpType(req.WalletId, x.WalletManagerCaller, x.ProjectManagerCaller)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot retrieve opType: %w", err)
	}
	instructionId, err := pmwpaymentutils.GenerateInstructionId(req.WalletId, opType, req.Nonce)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot generate instruction instruction id: %w", err)
	}
	// Event log
	eventHash, err := decoder.GetTeeInstructionsSentEventSignature(x.config.ParsedTeeInstructionsABI)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	chainLog, err := x.repo.FetchInstructionLog(ctx, eventHash, instructionId)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Decode event data
	paymentMessage, err := decoder.DecodeTeeInstructionsSentEventData(chainLog, x.config.ParsedTeeInstructionsABI)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Query underlying chain for transaction
	dbTransaction, err := x.repo.GetTransactionBySourceAndSequence(ctx, repo.ChainQuery{SourceAddress: paymentMessage.SenderAddress, Nonce: req.Nonce})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("transaction not found")
		}
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Parse transaction response JSON into structured data
	rawTransactionData, err := x.parseRawTransactionData(dbTransaction.Response)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Validate transaction and build response
	resp, err := builder.BuildPaymentStatusResponse(rawTransactionData, paymentMessage, dbTransaction)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	return resp, nil
}

func (x *XRPVerifier) parseRawTransactionData(response string) (xrptypes.RawTransactionData, error) {
	var rawTransactionData xrptypes.RawTransactionData
	err := json.Unmarshal([]byte(response), &rawTransactionData)
	if err != nil {
		logger.Errorf("failed to unmarshal XRP transaction response: %v, response: %s", err, response)
		return rawTransactionData, err
	}
	if rawTransactionData.MetaData.TransactionResult == "" {
		return rawTransactionData, fmt.Errorf("missing transaction result in raw transaction data")
	}
	return rawTransactionData, nil
}
