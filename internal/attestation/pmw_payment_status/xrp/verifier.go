package xrpverifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	utils "github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/instruction_event"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/builder"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/repo"
	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type XRPVerifier struct {
	Repo   *repo.XRPRepository
	Config *config.PMWPaymentStatusConfig
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWPaymentStatusRequestBody) (connector.IPMWPaymentStatusResponseBody, error) {
	// Build instruction ID
	opType, err := utils.StringToBytes32(fmt.Sprintf("F_%s", x.Config.SourceIDPair.SourceID))
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot retrieve opType: %w", err)
	}
	instructionID, err := generateInstructionID(req.WalletId, opType, req.Nonce)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, fmt.Errorf("cannot generate instruction instruction id: %w", err)
	}
	// Event log
	eventHash, err := teeinstruction.GetTeeInstructionsSentEventSignature(x.Config.ParsedTeeInstructionsABI)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	chainLog, err := x.Repo.FetchInstructionLog(ctx, eventHash, instructionID)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Decode event data
	paymentMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(chainLog, x.Config.ParsedTeeInstructionsABI)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, err
	}
	// Query underlying chain for transaction
	dbTransaction, err := x.Repo.GetTransactionBySourceAndSequence(ctx, repo.ChainQuery{SourceAddress: paymentMessage.SenderAddress, Nonce: req.Nonce})
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

func (x *XRPVerifier) parseRawTransactionData(response string) (types.RawTransactionData, error) {
	var rawTransactionData types.RawTransactionData
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

func generateInstructionID(walletID, opType [32]byte, nonce uint64) (common.Hash, error) {
	PAY, err := utils.StringToBytes32(string(op.Pay))
	if err != nil {
		return common.Hash{}, err
	}
	var nonceByte common.Hash
	nonceBig := big.NewInt(int64(nonce))
	copy(nonceByte[:], common.LeftPadBytes((nonceBig).Bytes(), utils.Bytes32Size))

	buf := new(bytes.Buffer)
	buf.Write(opType[:])
	buf.Write(PAY[:])
	buf.Write(walletID[:])
	buf.Write(nonceByte[:])
	instructionID := crypto.Keccak256Hash(buf.Bytes())
	return instructionID, nil
}
