package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/events"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"gorm.io/gorm"
)

type DBRepo struct {
	db              *gorm.DB
	cChainDb        *gorm.DB
	contractAddress string // lowercase hex, no 0x prefix — matches indexer storage format
}

// NewDBRepo constructs a DBRepo. contractAddress is the canonical contract that emits
// TeeInstructionsSent events; lookups from other addresses are treated as not-found.
func NewDBRepo(db, cChainDb *gorm.DB, contractAddress common.Address) *DBRepo {
	return &DBRepo{
		db:              db,
		cChainDb:        cChainDb,
		contractAddress: removeHexPrefix(strings.ToLower(contractAddress.Hex())),
	}
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
		Where("address = ? AND topic0 = ? AND topic1 = ? AND topic2 IN (?)",
			r.contractAddress,
			removeHexPrefix(eventHash),
			removeHexPrefix(common.HexToHash("").String()),
			topic2Values).
		Order("block_number ASC, log_index ASC").
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
			key := chainLog.Topics[2]
			if _, exists := result[key]; exists {
				return nil, fmt.Errorf("duplicate logs for instruction %s, eventHash %s: %w", key.Hex(), eventHash, paymentdb.ErrDatabase)
			}
			result[key] = chainLog
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
// The contract emits TeeInstructionsSent exactly once per instruction, so (topic0, topic1, topic2)
// is expected to be unique. Fetch up to two matches and surface a duplicate as an explicit error
// instead of silently selecting one arbitrarily.
func (r *DBRepo) FetchInstructionLog(ctx context.Context, eventHash string, instructionID common.Hash) (*InstructionLogResult, error) {
	var dbLogs []database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("address = ? AND topic0 = ? AND topic1 = ? AND topic2 = ?",
			r.contractAddress,
			removeHexPrefix(eventHash),
			removeHexPrefix(common.HexToHash("").String()),
			removeHexPrefix(instructionID.Hex())).
		Order("block_number ASC, log_index ASC").
		Limit(2).
		Find(&dbLogs).Error
	if err != nil {
		return nil, fmt.Errorf("cannot fetch log for instruction %s, eventHash %s: %w: %w", instructionID.Hex(), eventHash, paymentdb.ErrDatabase, err)
	}
	if len(dbLogs) == 0 {
		return nil, fmt.Errorf("cannot fetch log for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, paymentdb.ErrRecordNotFound)
	}
	if len(dbLogs) > 1 {
		return nil, fmt.Errorf("duplicate logs for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, paymentdb.ErrDatabase)
	}
	chainLog, err := events.ConvertDatabaseLogToChainLog(dbLogs[0])
	if err != nil {
		return nil, err
	}
	return &InstructionLogResult{Log: chainLog, BlockTimestamp: dbLogs[0].Timestamp}, nil
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
		if _, exists := result[tx.Sequence]; exists {
			return nil, fmt.Errorf("duplicate transactions for source %s, sequence %d: %w", sourceAddress, tx.Sequence, paymentdb.ErrDatabase)
		}
		result[tx.Sequence] = tx
	}
	return result, nil
}

func removeHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}
