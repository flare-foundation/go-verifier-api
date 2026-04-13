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

type ChainQuery struct {
	SourceAddress string
	Nonce         uint64
}

type DBRepo struct {
	db       *gorm.DB
	cChainDb *gorm.DB
}

func NewDBRepo(db, cChainDb *gorm.DB) *DBRepo {
	return &DBRepo{db: db, cChainDb: cChainDb}
}

func (r *DBRepo) FetchInstructionLog(ctx context.Context, eventHash string, instructionID common.Hash) (*types.Log, error) {
	// The contract emits TeeInstructionsSent exactly once per instruction, and nonces are
	// monotonic per account, so (topic0, topic1, topic2) is expected to be unique. Fetch up
	// to two matches ordered by canonical chain position so a hypothetical indexer
	// duplicate surfaces as an explicit error instead of a silently arbitrary pick.
	var dbLogs []database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("topic0 = ? AND topic1 = ? AND topic2 = ?",
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

func (r *DBRepo) FetchTransactionBySourceAndSequence(ctx context.Context, query ChainQuery) (DBTransaction, error) {
	var tx DBTransaction
	err := r.db.WithContext(ctx).
		Where("source_address = ? AND sequence = ?", query.SourceAddress, query.Nonce).
		First(&tx).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DBTransaction{}, fmt.Errorf("cannot fetch transaction for source %s, nonce %d: %w", query.SourceAddress, query.Nonce, ErrRecordNotFound)
		}
		return DBTransaction{}, fmt.Errorf("cannot fetch transaction for source %s, nonce %d: %w: %w", query.SourceAddress, query.Nonce, ErrDatabase, err)
	}
	return tx, nil
}

func removeHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}
