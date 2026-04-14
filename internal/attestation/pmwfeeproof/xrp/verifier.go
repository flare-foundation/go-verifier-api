package xrpverifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

var MaxNonceRange uint64 = 200

var (
	ErrNonceRangeTooLarge = errors.New("nonce range too large")
	ErrMissingPayEvent    = errors.New("missing pay event for nonce")
	ErrMissingTransaction = errors.New("missing transaction for nonce")
)

type XRPVerifier struct {
	Repo   *feeproofdb.DBRepo
	Config *config.PMWFeeProofConfig
}

func NewXRPVerifier(cfg *config.PMWFeeProofConfig, xrpDB, cChainDB *gorm.DB) *XRPVerifier {
	return &XRPVerifier{
		Repo:   feeproofdb.NewDBRepo(xrpDB, cChainDB, cfg.TeeInstructionsContractAddress),
		Config: cfg,
	}
}

func (x *XRPVerifier) Verify(ctx context.Context, req connector.IPMWFeeProofRequestBody) (connector.IPMWFeeProofResponseBody, error) {
	var zero connector.IPMWFeeProofResponseBody

	if req.ToNonce < req.FromNonce {
		return zero, fmt.Errorf("toNonce (%d) < fromNonce (%d): %w", req.ToNonce, req.FromNonce, ErrNonceRangeTooLarge)
	}
	if req.ToNonce-req.FromNonce+1 > MaxNonceRange {
		return zero, fmt.Errorf("nonce range %d exceeds max %d: %w", req.ToNonce-req.FromNonce+1, MaxNonceRange, ErrNonceRangeTooLarge)
	}

	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(x.Config.ParsedTeeInstructionsABI)
	if err != nil {
		return zero, err
	}

	sourceID := x.Config.SourceIDPair.SourceIDEncoded

	// Build nonce list and pay instruction IDs.
	nonceCount := int(req.ToNonce - req.FromNonce + 1)
	nonces := make([]uint64, nonceCount)
	payIDs := make([]common.Hash, nonceCount)
	for i := range nonceCount {
		nonce := req.FromNonce + uint64(i)
		nonces[i] = nonce
		id, err := instruction.GeneratePayInstructionID(req.OpType, sourceID, req.SenderAddress, nonce)
		if err != nil {
			return zero, fmt.Errorf("cannot generate pay instruction ID for nonce %d: %w", nonce, err)
		}
		payIDs[i] = id
	}

	// Batch fetch pay events.
	payLogs, err := x.Repo.FetchInstructionLogs(ctx, eventHash, payIDs)
	if err != nil {
		return zero, fmt.Errorf("cannot fetch pay events: %w", err)
	}

	estimatedFee, err := x.computeEstimatedFee(ctx, req, eventHash, sourceID, nonces, payIDs, payLogs)
	if err != nil {
		return zero, err
	}

	actualFee, err := x.computeActualFee(ctx, req.SenderAddress, nonces)
	if err != nil {
		return zero, err
	}

	return connector.IPMWFeeProofResponseBody{
		ActualFee:    actualFee,
		EstimatedFee: estimatedFee,
	}, nil
}

// computeEstimatedFee verifies all nonces have pay events and sums the estimated fees
// including residuals from reissue events.
func (x *XRPVerifier) computeEstimatedFee(ctx context.Context, req connector.IPMWFeeProofRequestBody, eventHash string, sourceID [32]byte, nonces []uint64, payIDs []common.Hash, payLogs map[common.Hash]*ethtypes.Log) (*big.Int, error) {
	estimatedFee := new(big.Int)
	for i, nonce := range nonces {
		payLog, ok := payLogs[payIDs[i]]
		if !ok {
			return nil, fmt.Errorf("nonce %d: %w", nonce, ErrMissingPayEvent)
		}

		payMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(payLog, x.Config.ParsedTeeInstructionsABI, op.Pay)
		if err != nil {
			return nil, fmt.Errorf("cannot decode pay event for nonce %d: %w", nonce, err)
		}

		payMaxFee := payMessage.MaxFee
		estimatedFee.Add(estimatedFee, payMaxFee)

		// Iteratively fetch reissue events for this nonce.
		for reissueNum := uint64(0); ; reissueNum++ {
			reissueID, err := instruction.GenerateReissueInstructionID(req.OpType, sourceID, req.SenderAddress, nonce, reissueNum)
			if err != nil {
				return nil, fmt.Errorf("cannot generate reissue instruction ID for nonce %d, reissue %d: %w", nonce, reissueNum, err)
			}

			reissueResult, err := x.Repo.FetchInstructionLog(ctx, eventHash, reissueID)
			if err != nil {
				if errors.Is(err, paymentdb.ErrRecordNotFound) {
					break // No more reissues for this nonce.
				}
				return nil, fmt.Errorf("cannot fetch reissue event for nonce %d, reissue %d: %w", nonce, reissueNum, err)
			}

			// Skip reissue events after untilTimestamp (inclusive).
			if reissueResult.BlockTimestamp > req.UntilTimestamp {
				break
			}

			reissueMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(reissueResult.Log, x.Config.ParsedTeeInstructionsABI, op.Reissue)
			if err != nil {
				return nil, fmt.Errorf("cannot decode reissue event for nonce %d, reissue %d: %w", nonce, reissueNum, err)
			}

			// Residual: max(0, reissue_maxFee - pay_maxFee)
			residual := new(big.Int).Sub(reissueMessage.MaxFee, payMaxFee)
			if residual.Sign() > 0 {
				estimatedFee.Add(estimatedFee, residual)
			}
		}
	}
	return estimatedFee, nil
}

// computeActualFee fetches XRP transactions for the nonce range and sums their fees.
func (x *XRPVerifier) computeActualFee(ctx context.Context, senderAddress string, nonces []uint64) (*big.Int, error) {
	txMap, err := x.Repo.FetchTransactionsBySourceAndSequences(ctx, senderAddress, nonces)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch transactions: %w", err)
	}

	actualFee := new(big.Int)
	for _, nonce := range nonces {
		tx, ok := txMap[nonce]
		if !ok {
			return nil, fmt.Errorf("nonce %d: %w", nonce, ErrMissingTransaction)
		}

		fee, err := parseTxFee(tx.Response)
		if err != nil {
			return nil, fmt.Errorf("cannot parse fee for nonce %d: %w", nonce, err)
		}
		actualFee.Add(actualFee, fee)
	}
	return actualFee, nil
}

func parseTxFee(response string) (*big.Int, error) {
	var raw struct {
		Fee string `json:"Fee"`
	}
	if err := json.Unmarshal([]byte(response), &raw); err != nil {
		logger.Errorf("Cannot unmarshal XRP transaction response for fee: %v", err)
		return nil, fmt.Errorf("cannot unmarshal transaction response: %w", err)
	}
	if raw.Fee == "" {
		return nil, errors.New("missing Fee in transaction response")
	}
	fee, ok := new(big.Int).SetString(raw.Fee, 10)
	if !ok {
		return nil, fmt.Errorf("cannot parse Fee %q as integer", raw.Fee)
	}
	return fee, nil
}
