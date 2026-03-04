package db

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newClosedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, CloseDB(db))
	return db
}

func TestFetchInstructionLog_DatabaseError(t *testing.T) {
	repo := NewDBRepo(nil, newClosedDB(t))
	_, err := repo.FetchInstructionLog(context.Background(), "0xabc", common.HexToHash("0x1"))
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch log for instruction")
}

func TestGetTransactionBySourceAndSequence_DatabaseError(t *testing.T) {
	repo := NewDBRepo(newClosedDB(t), nil)
	_, err := repo.GetTransactionBySourceAndSequence(context.Background(), ChainQuery{SourceAddress: "addr", Nonce: 1})
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch transaction for source")
}
