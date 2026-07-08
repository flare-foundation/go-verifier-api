package xrpverifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwnonce"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/helper"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

// MaxBatchRange is the maximum number of payments a single fee-proof request
// may span — a DoS backstop against a request that would fan out to unbounded
// DB work. It is an immutable const so no importer can silently raise the
// ceiling; a verifier instance may override it via the unexported
// maxBatchRange field (used only by the scaling benchmark).
const MaxBatchRange uint64 = 200

// MaxReissuesPerPayment caps how many reissue events the verifier will scan
// per payment. The contract has no on-chain cap on reissues; only a
// per-batch timing gate. A polluted indexer could therefore make a single
// request trigger arbitrary DB work. This cap is a defense-in-depth
// backstop — well above any realistic retry count (typical wallets: 0–3
// reissues, pathological: 5–10).
const MaxReissuesPerPayment uint64 = 32

var (
	ErrBatchRangeTooLarge   = errors.New("batch range too large")
	ErrMissingPayEvent      = errors.New("missing pay event for paymentId")
	ErrMissingTransaction   = errors.New("missing transaction for paymentId")
	ErrReissueLimitExceeded = errors.New("reissue scan limit exceeded")
)

type XRPVerifier struct {
	Repo   *feeproofdb.DBRepo
	Config *config.PMWFeeProofConfig
	// Binder binds each payment's XRP sequence to its paymentId via on-chain
	// initialNonce. An interface so tests can substitute a stub.
	Binder pmwnonce.SequenceVerifier
	// maxBatchRange overrides the batch-size cap for this instance; zero means
	// use the MaxBatchRange default. Only the scaling benchmark sets it.
	maxBatchRange uint64
}

func NewXRPVerifier(cfg *config.PMWFeeProofConfig, xrpDB, cChainDB *gorm.DB) (*XRPVerifier, error) {
	binder, err := pmwnonce.NewOnChainBinder(cfg.RPCURL, cfg.TeePaymentsContractAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot create initial-nonce binder: %w", err)
	}
	return &XRPVerifier{
		Repo:   feeproofdb.NewDBRepo(xrpDB, cChainDB, cfg.FlareTeeManagerContractAddress),
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

func (x *XRPVerifier) Verify(ctx context.Context, req fdc2.IPMWFeeProofRequestBody) (fdc2.IPMWFeeProofResponseBody, error) {
	var zero fdc2.IPMWFeeProofResponseBody

	if req.BatchCount == 0 {
		return zero, fmt.Errorf("batchCount must be greater than 0: %w", ErrBatchRangeTooLarge)
	}
	maxBatch := x.maxBatchRange
	if maxBatch == 0 {
		maxBatch = MaxBatchRange
	}
	if req.BatchCount > maxBatch {
		return zero, fmt.Errorf("batchCount %d exceeds max size %d: %w", req.BatchCount, maxBatch, ErrBatchRangeTooLarge)
	}
	// Guard against overflow of the inclusive upper bound FirstPaymentId+BatchCount-1.
	if req.FirstPaymentId > math.MaxUint64-(req.BatchCount-1) {
		return zero, fmt.Errorf("paymentId range from %d count %d overflows uint64: %w", req.FirstPaymentId, req.BatchCount, ErrBatchRangeTooLarge)
	}

	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(x.Config.ParsedTeeInstructionsABI)
	if err != nil {
		return zero, err
	}

	sourceID := x.Config.SourceIDPair.SourceIDEncoded

	// Build paymentId list and pay instruction IDs.
	paymentIDs := make([]uint64, req.BatchCount)
	payIDs := make([]common.Hash, req.BatchCount)
	for i := range int(req.BatchCount) {
		paymentId := req.FirstPaymentId + uint64(i)
		paymentIDs[i] = paymentId
		id, err := instruction.GeneratePayInstructionID(req.OpType, sourceID, req.SenderAddress, paymentId)
		if err != nil {
			return zero, fmt.Errorf("cannot generate pay instruction ID for paymentId %d: %w", paymentId, err)
		}
		payIDs[i] = id
	}

	// Batch fetch pay events.
	payLogs, err := x.Repo.FetchInstructionLogs(ctx, eventHash, payIDs)
	if err != nil {
		return zero, fmt.Errorf("cannot fetch pay events: %w", err)
	}

	// Estimated fee from C-chain events; sequences are each payment's XRP Sequence,
	// read from the decoded event (the request no longer carries them).
	estimatedFee, sequences, err := x.computeEstimatedFee(ctx, req, eventHash, sourceID, paymentIDs, payIDs, payLogs)
	if err != nil {
		return zero, err
	}

	actualFee, err := x.computeActualFee(ctx, req.SenderAddress, paymentIDs, sequences)
	if err != nil {
		return zero, err
	}

	// Defense in depth: the per-value drops ceilings already bound each summand
	// far below 2^256, but never hand a >uint256 total to the ABI encoder, which
	// would silently wrap mod 2^256 and could make a tampered fee read as
	// reconciling. A sum this large can only be corrupt data.
	if estimatedFee.BitLen() > 256 || actualFee.BitLen() > 256 {
		return zero, fmt.Errorf("fee sum exceeds uint256 (estimated bits=%d, actual bits=%d): %w", estimatedFee.BitLen(), actualFee.BitLen(), paymentdb.ErrDatabase)
	}

	return fdc2.IPMWFeeProofResponseBody{
		LastPaymentId: req.FirstPaymentId + req.BatchCount - 1,
		ActualFee:     actualFee,
		EstimatedFee:  estimatedFee,
	}, nil
}

// computeEstimatedFee verifies all paymentIds have pay events and sums the estimated
// fees including residuals from reissue events. It also returns each payment's XRP
// Sequence (read from the decoded event), used to fetch the actual transactions.
func (x *XRPVerifier) computeEstimatedFee(ctx context.Context, req fdc2.IPMWFeeProofRequestBody, eventHash string, sourceID [32]byte, paymentIDs []uint64, payIDs []common.Hash, payLogs map[common.Hash]*ethtypes.Log) (*big.Int, []uint64, error) {
	estimatedFee := new(big.Int)
	sequences := make([]uint64, len(paymentIDs))
	for i, paymentId := range paymentIDs {
		payLog, ok := payLogs[payIDs[i]]
		if !ok {
			return nil, nil, fmt.Errorf("paymentId %d: %w", paymentId, ErrMissingPayEvent)
		}

		payMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(payLog, x.Config.ParsedTeeInstructionsABI, op.Pay, req.OpType)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot decode pay event for paymentId %d: %w", paymentId, err)
		}
		// Bind the decoded event to the request: instructionId commits to these
		// fields, so a mismatch is C-chain index inconsistency.
		if err := paymentdb.CheckInstructionConsistency(payMessage, common.Hash(sourceID), req.SenderAddress, paymentId); err != nil {
			return nil, nil, fmt.Errorf("paymentId %d: %w", paymentId, err)
		}
		// Bind the pay event's XRP sequence to the value the contract
		// deterministically assigns this paymentId (initialNonce + paymentId - 1),
		// read on-chain. The instruction-ID topic does not commit to the sequence,
		// so without this a compromised indexer could substitute a foreign sequence
		// (and thus a foreign transaction's fee) for a paymentId.
		if err := x.Binder.VerifySequence(ctx, common.Hash(sourceID), req.SenderAddress, paymentId, payMessage.Nonce); err != nil {
			return nil, nil, fmt.Errorf("paymentId %d: %w", paymentId, err)
		}
		// XRP Sequence used to locate the actual transaction.
		sequences[i] = payMessage.Nonce

		payMaxFee := payMessage.MaxFee
		if helper.ExceedsMaxXRPDrops(payMaxFee) {
			return nil, nil, fmt.Errorf("paymentId %d: pay maxFee %s exceeds total supply in drops: %w", paymentId, payMaxFee, paymentdb.ErrDatabase)
		}
		estimatedFee.Add(estimatedFee, payMaxFee)

		terminatedEarly := false
		for reissueNum := uint64(1); reissueNum <= MaxReissuesPerPayment; reissueNum++ {
			reissueID, err := instruction.GenerateReissueInstructionID(req.OpType, sourceID, req.SenderAddress, paymentId, reissueNum)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot generate reissue instruction ID for paymentId %d, reissue %d: %w", paymentId, reissueNum, err)
			}

			reissueResult, err := x.Repo.FetchInstructionLog(ctx, eventHash, reissueID)
			if errors.Is(err, paymentdb.ErrRecordNotFound) {
				terminatedEarly = true
				break
			}
			if err != nil {
				return nil, nil, fmt.Errorf("cannot fetch reissue event for paymentId %d, reissue %d: %w", paymentId, reissueNum, err)
			}
			if reissueResult.BlockTimestamp > req.UntilTimestamp {
				terminatedEarly = true
				break
			}

			reissueMessage, err := teeinstruction.DecodeTeeInstructionsSentEventData(reissueResult.Log, x.Config.ParsedTeeInstructionsABI, op.Reissue, req.OpType)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot decode reissue event for paymentId %d, reissue %d: %w", paymentId, reissueNum, err)
			}
			// Reissue instructionId commits to (sourceId, sender, paymentId) too; bind
			// the decoded event back to the request.
			if err := paymentdb.CheckInstructionConsistency(reissueMessage, common.Hash(sourceID), req.SenderAddress, paymentId); err != nil {
				return nil, nil, fmt.Errorf("paymentId %d, reissue %d: %w", paymentId, reissueNum, err)
			}

			if helper.ExceedsMaxXRPDrops(reissueMessage.MaxFee) {
				return nil, nil, fmt.Errorf("paymentId %d, reissue %d: maxFee %s exceeds total supply in drops: %w", paymentId, reissueNum, reissueMessage.MaxFee, paymentdb.ErrDatabase)
			}
			// Residual: max(0, reissue_maxFee - pay_maxFee)
			residual := new(big.Int).Sub(reissueMessage.MaxFee, payMaxFee)
			if residual.Sign() > 0 {
				estimatedFee.Add(estimatedFee, residual)
			}
		}
		if terminatedEarly {
			continue
		}
		// Loop ran through MaxReissuesPerPayment. Probe the next reissueNumber to
		// distinguish "exactly cap reissues exist" (legitimate, accept) from
		// ">cap exist" (would silently undercount estimatedFee, reject).
		nextReissueNum := MaxReissuesPerPayment + 1
		nextID, err := instruction.GenerateReissueInstructionID(req.OpType, sourceID, req.SenderAddress, paymentId, nextReissueNum)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot generate reissue instruction ID for paymentId %d, reissue %d: %w", paymentId, nextReissueNum, err)
		}
		nextResult, err := x.Repo.FetchInstructionLog(ctx, eventHash, nextID)
		if errors.Is(err, paymentdb.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("cannot probe reissue cap for paymentId %d: %w", paymentId, err)
		}
		// The probe event exists, but only counts against the cap if it falls within
		// the same untilTimestamp window the scan applies. A reissue past
		// untilTimestamp is outside this batch's window, so within the window there
		// are exactly MaxReissuesPerPayment reissues — legitimate, accept.
		if nextResult.BlockTimestamp > req.UntilTimestamp {
			continue
		}
		return nil, nil, fmt.Errorf("paymentId %d: %w (cap %d)", paymentId, ErrReissueLimitExceeded, MaxReissuesPerPayment)
	}
	return estimatedFee, sequences, nil
}

// computeActualFee fetches the XRP transactions for the given sequences (one per
// payment, derived from the decoded events) and sums their fees.
func (x *XRPVerifier) computeActualFee(ctx context.Context, senderAddress string, paymentIDs []uint64, sequences []uint64) (*big.Int, error) {
	txMap, err := x.Repo.FetchTransactionsBySourceAndSequences(ctx, senderAddress, sequences)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch transactions: %w", err)
	}

	actualFee := new(big.Int)
	for i, sequence := range sequences {
		tx, ok := txMap[sequence]
		if !ok {
			return nil, fmt.Errorf("paymentId %d (sequence %d): %w", paymentIDs[i], sequence, ErrMissingTransaction)
		}

		// Bind the Response blob to its DB row before trusting its Fee: a partial
		// write or targeted tamper could otherwise feed a foreign transaction's fee
		// into the sum. Mirrors PMWPaymentStatus's check (db.CheckRowConsistency).
		if err := checkTxRowConsistency(tx); err != nil {
			return nil, fmt.Errorf("paymentId %d (sequence %d): %w", paymentIDs[i], sequence, err)
		}
		fee, err := parseTxFee(tx.Response)
		if err != nil {
			return nil, fmt.Errorf("cannot parse fee for paymentId %d (sequence %d): %w", paymentIDs[i], sequence, err)
		}
		actualFee.Add(actualFee, fee)
	}
	return actualFee, nil
}

const maxResponseSize = 1 << 20

// checkTxRowConsistency confirms the transaction's Response JSON describes the
// same row as the canonical DB columns (hash/account/sequence) before its Fee
// is used.
func checkTxRowConsistency(tx paymentdb.DBTransaction) error {
	if len(tx.Response) > maxResponseSize {
		return fmt.Errorf("XRP transaction response too large: %d bytes (max %d): %w", len(tx.Response), maxResponseSize, paymentdb.ErrDatabase)
	}
	var id struct {
		Account  string `json:"Account"`
		Sequence uint64 `json:"Sequence"`
		Hash     string `json:"hash"`
	}
	if err := json.Unmarshal([]byte(tx.Response), &id); err != nil {
		return fmt.Errorf("cannot unmarshal transaction response: %w", err)
	}
	return paymentdb.CheckRowConsistency(id.Hash, id.Account, id.Sequence, tx)
}

func parseTxFee(response string) (*big.Int, error) {
	if len(response) > maxResponseSize {
		return nil, fmt.Errorf("XRP transaction response too large: %d bytes (max %d): %w", len(response), maxResponseSize, paymentdb.ErrDatabase)
	}
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
	fee, err := helper.ParseNonNegativeBigInt(raw.Fee)
	if err != nil {
		return nil, fmt.Errorf("cannot parse Fee %q: %w", raw.Fee, err)
	}
	// Fail closed on an impossible fee: a drops value above the total XRP supply
	// cannot be real data (corrupt/tampered indexer row), and summing it would
	// forge the fee proof.
	if helper.ExceedsMaxXRPDrops(fee) {
		return nil, fmt.Errorf("XRP Fee %s exceeds total supply in drops (%d): %w", fee, helper.MaxXRPDrops, paymentdb.ErrDatabase)
	}
	return fee, nil
}
