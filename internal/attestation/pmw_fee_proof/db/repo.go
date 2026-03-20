package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/events"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/db"
	"gorm.io/gorm"
)

type DBRepo struct {
	db       *gorm.DB
	cChainDb *gorm.DB
}

func NewDBRepo(db, cChainDb *gorm.DB) *DBRepo {
	return &DBRepo{db: db, cChainDb: cChainDb}
}

// FetchInstructionLogs fetches logs for multiple instruction IDs in a single query.
func (r *DBRepo) FetchInstructionLogs(ctx context.Context, eventHash string, instructionIDs []common.Hash) (map[common.Hash]*types.Log, error) {
	if len(instructionIDs) == 0 {
		return make(map[common.Hash]*types.Log), nil
	}

	topic2Values := make([]string, len(instructionIDs))
	for i, id := range instructionIDs {
		topic2Values[i] = removeHexPrefix(id.Hex())
	}

	var dbLogs []database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("topic0 = ? AND topic1 = ? AND topic2 IN (?)",
			removeHexPrefix(eventHash),
			removeHexPrefix(common.HexToHash("").String()),
			topic2Values).
		Find(&dbLogs).Error
	if err != nil {
		return nil, fmt.Errorf("cannot fetch instruction logs, eventHash %s: %w: %w", eventHash, paymentdb.ErrDatabase, err)
	}

	result := make(map[common.Hash]*types.Log, len(dbLogs))
	for _, dbLog := range dbLogs {
		chainLog, err := events.ConvertDatabaseLogToChainLog(dbLog)
		if err != nil {
			return nil, fmt.Errorf("cannot convert log: %w", err)
		}
		if len(chainLog.Topics) >= 3 {
			result[chainLog.Topics[2]] = chainLog
		}
	}
	return result, nil
}

// InstructionLogResult holds a decoded chain log and its block timestamp.
type InstructionLogResult struct {
	Log            *types.Log
	BlockTimestamp uint64
}

// FetchInstructionLog fetches a single instruction log. Used for iterative reissue lookups.
func (r *DBRepo) FetchInstructionLog(ctx context.Context, eventHash string, instructionID common.Hash) (*InstructionLogResult, error) {
	var dbLog database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("topic0 = ? AND topic1 = ? AND topic2 = ?",
			removeHexPrefix(eventHash),
			removeHexPrefix(common.HexToHash("").String()),
			removeHexPrefix(instructionID.Hex())).
		First(&dbLog).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("cannot fetch log for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, paymentdb.ErrRecordNotFound)
		}
		return nil, fmt.Errorf("cannot fetch log for instruction %s, eventHash %s: %w: %w", instructionID.Hex(), eventHash, paymentdb.ErrDatabase, err)
	}
	chainLog, err := events.ConvertDatabaseLogToChainLog(dbLog)
	if err != nil {
		return nil, err
	}
	return &InstructionLogResult{Log: chainLog, BlockTimestamp: dbLog.Timestamp}, nil
}

// FetchTransactionsBySourceAndSequences fetches XRP transactions for multiple nonces in a single query.
func (r *DBRepo) FetchTransactionsBySourceAndSequences(ctx context.Context, sourceAddress string, nonces []uint64) (map[uint64]paymentdb.DBTransaction, error) {
	if len(nonces) == 0 {
		return make(map[uint64]paymentdb.DBTransaction), nil
	}

	var txs []paymentdb.DBTransaction
	err := r.db.WithContext(ctx).
		Where("source_address = ? AND sequence IN (?)", sourceAddress, nonces).
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("cannot fetch transactions for source %s: %w: %w", sourceAddress, paymentdb.ErrDatabase, err)
	}

	result := make(map[uint64]paymentdb.DBTransaction, len(txs))
	for _, tx := range txs {
		result[tx.Sequence] = tx
	}
	return result, nil
}

func removeHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}
