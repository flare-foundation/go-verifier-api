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
	"gorm.io/gorm"
)

var (
	// ErrRecordNotFound indicates the requested record does not exist in the database.
	ErrRecordNotFound = errors.New("record not found")
	// ErrDatabase indicates a database infrastructure failure (connection, timeout, etc.).
	ErrDatabase = errors.New("database error")
)

// maxInstructionLogs bounds how many event rows one instruction id may load. A
// batch's payments share an instruction id, so multiple rows are expected, but the
// count is bounded by real batch sizes; a larger set signals a corrupt/hostile
// index and is refused rather than loaded and decoded unboundedly.
const maxInstructionLogs = 4096

type ChainQuery struct {
	SourceAddress string
	Nonce         uint64
}

type DBRepo struct {
	db              *gorm.DB
	cChainDb        *gorm.DB
	contractAddress string // lowercase hex, no 0x prefix — matches indexer storage format
}

// NewDBRepo constructs a DBRepo. contractAddress is the canonical contract that emits
// TeeInstructionsSent events. Lookups that do not match this address are treated as
// "record not found".
func NewDBRepo(db, cChainDb *gorm.DB, contractAddress common.Address) *DBRepo {
	return &DBRepo{
		db:              db,
		cChainDb:        cChainDb,
		contractAddress: normalizeAddress(contractAddress),
	}
}

func (r *DBRepo) FetchInstructionLog(ctx context.Context, eventHash string, instructionID common.Hash) (*types.Log, error) {
	// The contract emits TeeInstructionsSent exactly once per instruction, and nonces are
	// monotonic per account, so (topic0, topic1, topic2) is expected to be unique. Fetch up
	// to two matches ordered by canonical chain position so a hypothetical indexer
	// duplicate surfaces as an explicit error instead of a silently arbitrary pick.
	var dbLogs []database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("address = ? AND topic0 = ? AND topic1 = ? AND topic2 = ?",
			r.contractAddress,
			removeHexPrefix(eventHash),
			removeHexPrefix(common.HexToHash("").String()), // Only checking for extensionID = 0.
			removeHexPrefix(instructionID.Hex())).
		Order("block_number ASC, log_index ASC").
		Limit(2).
		Find(&dbLogs).Error
	if err != nil {
		return nil, fmt.Errorf("cannot fetch log for instruction %s, eventHash %s: %w: %w", instructionID.Hex(), eventHash, ErrDatabase, err)
	}
	if len(dbLogs) == 0 {
		return nil, fmt.Errorf("cannot fetch log for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, ErrRecordNotFound)
	}
	if len(dbLogs) > 1 {
		return nil, fmt.Errorf("duplicate logs for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, ErrDatabase)
	}
	return events.ConvertDatabaseLogToChainLog(dbLogs[0])
}

// FetchLogsByInstructionTopic1 fetches every log whose FIRST indexed argument
// is the instruction id.
//
// The diamond indexes extensionId first and the instruction id second; the CSP
// channel's PaymentBatched has no extension and indexes the instruction id
// first. Same question, different topic position — which is this method's
// business rather than something each caller patches around.
func (r *DBRepo) FetchLogsByInstructionTopic1(ctx context.Context, eventHash string, instructionID common.Hash) ([]*types.Log, error) {
	var dbLogs []database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("address = ? AND topic0 = ? AND topic1 = ?",
			r.contractAddress,
			removeHexPrefix(eventHash),
			removeHexPrefix(instructionID.Hex())).
		Order("block_number ASC, log_index ASC").
		Limit(maxInstructionLogs + 1). // +1 to detect an over-cap set rather than silently truncating
		Find(&dbLogs).Error
	if err != nil {
		return nil, fmt.Errorf("cannot fetch logs for instruction %s, eventHash %s: %w: %w", instructionID.Hex(), eventHash, ErrDatabase, err)
	}
	if len(dbLogs) == 0 {
		return nil, fmt.Errorf("cannot fetch logs for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, ErrRecordNotFound)
	}
	if len(dbLogs) > maxInstructionLogs {
		return nil, fmt.Errorf("instruction %s has more than %d logs; refusing to load an unbounded set: %w", instructionID.Hex(), maxInstructionLogs, ErrDatabase)
	}
	logs := make([]*types.Log, 0, len(dbLogs))
	for _, dbLog := range dbLogs {
		chainLog, err := events.ConvertDatabaseLogToChainLog(dbLog)
		if err != nil {
			return nil, err
		}
		logs = append(logs, chainLog)
	}
	return logs, nil
}

// FetchInstructionLogsForID fetches every TeeInstructionsSent log sharing one
// instruction ID. A Bitcoin batch's payments share a single instruction ID (it
// is derived from batchPaymentId), so — unlike the XRP path's FetchInstructionLog
// — multiple rows are expected here, one per payment in the batch, and duplicates
// are not an error. The caller filters the decoded messages by paymentId.
func (r *DBRepo) FetchInstructionLogsForID(ctx context.Context, eventHash string, instructionID common.Hash) ([]*types.Log, error) {
	var dbLogs []database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("address = ? AND topic0 = ? AND topic1 = ? AND topic2 = ?",
			r.contractAddress,
			removeHexPrefix(eventHash),
			removeHexPrefix(common.HexToHash("").String()), // Only checking for extensionID = 0.
			removeHexPrefix(instructionID.Hex())).
		Order("block_number ASC, log_index ASC").
		Limit(maxInstructionLogs + 1). // +1 to detect an over-cap set rather than silently truncating
		Find(&dbLogs).Error
	if err != nil {
		return nil, fmt.Errorf("cannot fetch logs for instruction %s, eventHash %s: %w: %w", instructionID.Hex(), eventHash, ErrDatabase, err)
	}
	if len(dbLogs) == 0 {
		return nil, fmt.Errorf("cannot fetch logs for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, ErrRecordNotFound)
	}
	if len(dbLogs) > maxInstructionLogs {
		return nil, fmt.Errorf("instruction %s has more than %d logs; refusing to load an unbounded set: %w", instructionID.Hex(), maxInstructionLogs, ErrDatabase)
	}
	logs := make([]*types.Log, 0, len(dbLogs))
	for _, dbLog := range dbLogs {
		chainLog, err := events.ConvertDatabaseLogToChainLog(dbLog)
		if err != nil {
			return nil, fmt.Errorf("cannot convert log for instruction %s: %w", instructionID.Hex(), err)
		}
		logs = append(logs, chainLog)
	}
	return logs, nil
}

// normalizeAddress returns the address in the lowercase, 0x-stripped form the indexer
// stores in database.Log.Address (varchar(40)).
func normalizeAddress(addr common.Address) string {
	return removeHexPrefix(strings.ToLower(addr.Hex()))
}

func (r *DBRepo) FetchTransactionBySourceAndSequence(ctx context.Context, query ChainQuery) (DBTransaction, error) {
	// (source_address, sequence) is consumed exactly once per validated XRPL ledger,
	// so this is expected to match at most one row. Fetch up to two (ordered by the
	// Hash primary key) so a hypothetical indexer duplicate surfaces as an explicit
	// error instead of a silently arbitrary pick — mirrors FetchInstructionLog and the
	// PMWFeeProof batch fetch, both of which fail closed on duplicates.
	var txs []DBTransaction
	err := r.db.WithContext(ctx).
		Where("source_address = ? AND sequence = ?", query.SourceAddress, query.Nonce).
		Order("hash ASC").
		Limit(2).
		Find(&txs).Error
	if err != nil {
		return DBTransaction{}, fmt.Errorf("cannot fetch transaction for source %s, nonce %d: %w: %w", query.SourceAddress, query.Nonce, ErrDatabase, err)
	}
	if len(txs) == 0 {
		return DBTransaction{}, fmt.Errorf("cannot fetch transaction for source %s, nonce %d: %w", query.SourceAddress, query.Nonce, ErrRecordNotFound)
	}
	if len(txs) > 1 {
		return DBTransaction{}, fmt.Errorf("duplicate transactions for source %s, nonce %d: %w", query.SourceAddress, query.Nonce, ErrDatabase)
	}
	return txs[0], nil
}

func removeHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}
