package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/model"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/events"
	"gorm.io/gorm"
)

type ChainQuery struct {
	SourceAddress string
	Nonce         uint64
}

type XRPRepository struct {
	db       *gorm.DB
	cChainDb *gorm.DB
}

func NewXRPRepository(db, cChainDb *gorm.DB) *XRPRepository {
	return &XRPRepository{db: db, cChainDb: cChainDb}
}

func (r *XRPRepository) FetchInstructionLog(ctx context.Context, eventHash string, instructionID common.Hash) (*types.Log, error) {
	var dbLog database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("topic0 = ? AND topic1 = ? AND topic2 = ?",
			removeHexPrefix(eventHash),
			removeHexPrefix(common.HexToHash("").String()), // Only checking for extensionID = 0.
			removeHexPrefix(instructionID.Hex())).
		First(&dbLog).Error
	if err != nil {
		return nil, fmt.Errorf("cannot fetch log for instruction %s, eventHash %s: %w", instructionID.Hex(), eventHash, err)
	}
	return events.ConvertDatabaseLogToChainLog(dbLog)
}

func (r *XRPRepository) GetTransactionBySourceAndSequence(ctx context.Context, query ChainQuery) (model.DBTransaction, error) {
	var tx model.DBTransaction
	err := r.db.WithContext(ctx).
		Where("source_address = ? AND sequence = ?", query.SourceAddress, query.Nonce).
		First(&tx).Error
	if err != nil {
		return model.DBTransaction{}, fmt.Errorf("cannot fetch transaction for source %s, nonce %d: %w", query.SourceAddress, query.Nonce, err)
	}
	return tx, nil
}

func removeHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}
