package repo

import (
	"context"
	"fmt"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/events"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/models"
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

func (r *XRPRepository) FetchInstructionLog(ctx context.Context, eventHash string, instructionId common.Hash) (*types.Log, error) {
	var dbLog database.Log
	err := r.cChainDb.WithContext(ctx).
		Where("topic0 = ? AND topic1 = ?", utils.RemoveHexPrefix(eventHash), utils.RemoveHexPrefix(instructionId.Hex())).
		First(&dbLog).Error
	if err != nil {
		return nil, fmt.Errorf("log not found for instruction %s", instructionId)
	}
	return events.ConvertDatabaseLogToChainLog(dbLog)
}

func (r *XRPRepository) GetTransactionBySourceAndSequence(ctx context.Context, query ChainQuery) (models.DBTransaction, error) {
	var tx models.DBTransaction
	err := r.db.WithContext(ctx).
		Where("source_address = ? AND sequence = ?", query.SourceAddress, query.Nonce).
		First(&tx).Error
	if err != nil {
		return models.DBTransaction{}, err
	}
	return tx, nil
}
