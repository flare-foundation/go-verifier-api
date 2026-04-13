package db

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/database"
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

func newMemoryDBWithLogs(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.Log{}))
	return db
}

func TestFetchInstructionLog_NotFound(t *testing.T) {
	repo := NewDBRepo(nil, newMemoryDBWithLogs(t))
	_, err := repo.FetchInstructionLog(context.Background(), "abc", common.HexToHash("0x1"))
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestFetchInstructionLog_DuplicateRowsAreRejected(t *testing.T) {
	cdb := newMemoryDBWithLogs(t)
	instructionID := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(instructionID.Hex(), "0x")

	// Insert two logs with identical (topic0, topic1, topic2).
	for i := range 2 {
		require.NoError(t, cdb.Create(&database.Log{
			Topic0:          eventHash,
			Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
			Topic2:          topic2,
			Data:            "",
			Address:         "addr",
			TransactionHash: strings.Repeat("0", 63) + string(rune('0'+i)),
			LogIndex:        uint64(i),
			BlockNumber:     uint64(10 + i),
			Timestamp:       1700000000,
		}).Error)
	}

	repo := NewDBRepo(nil, cdb)
	_, err := repo.FetchInstructionLog(context.Background(), eventHash, instructionID)
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "duplicate logs for instruction")
}

func TestFetchTransactionBySourceAndSequence_DatabaseError(t *testing.T) {
	repo := NewDBRepo(newClosedDB(t), nil)
	_, err := repo.FetchTransactionBySourceAndSequence(context.Background(), ChainQuery{SourceAddress: "addr", Nonce: 1})
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch transaction for source")
}
