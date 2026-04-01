package db

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/db"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newClosedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, paymentdb.CloseDB(db))
	return db
}

func newMemoryDB(t *testing.T, migrate ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	for _, m := range migrate {
		require.NoError(t, db.AutoMigrate(m))
	}
	return db
}

func TestFetchInstructionLogs_DatabaseError(t *testing.T) {
	repo := NewDBRepo(nil, newClosedDB(t))
	ids := []common.Hash{common.HexToHash("0x1")}
	_, err := repo.FetchInstructionLogs(context.Background(), "0xabc", ids)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch instruction logs")
}

func TestFetchInstructionLogs_EmptyIDs(t *testing.T) {
	repo := NewDBRepo(nil, nil)
	result, err := repo.FetchInstructionLogs(context.Background(), "0xabc", nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchInstructionLogs_NoResults(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	repo := NewDBRepo(nil, cchainDb)
	ids := []common.Hash{common.HexToHash("0x1")}
	result, err := repo.FetchInstructionLogs(context.Background(), "0xabc", ids)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchInstructionLog_DatabaseError(t *testing.T) {
	repo := NewDBRepo(nil, newClosedDB(t))
	_, err := repo.FetchInstructionLog(context.Background(), "0xabc", common.HexToHash("0x1"))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch log for instruction")
}

func TestFetchInstructionLog_RecordNotFound(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	repo := NewDBRepo(nil, cchainDb)
	_, err := repo.FetchInstructionLog(context.Background(), "0xabc", common.HexToHash("0x1"))
	require.ErrorIs(t, err, paymentdb.ErrRecordNotFound)
}

func TestFetchTransactionsBySourceAndSequences_DatabaseError(t *testing.T) {
	repo := NewDBRepo(newClosedDB(t), nil)
	_, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", []uint64{1, 2})
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch transactions for source")
}

func TestFetchTransactionsBySourceAndSequences_EmptyNonces(t *testing.T) {
	repo := NewDBRepo(nil, nil)
	result, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchTransactionsBySourceAndSequences_NoResults(t *testing.T) {
	sourceDb := newMemoryDB(t, &paymentdb.DBTransaction{})
	repo := NewDBRepo(sourceDb, nil)
	result, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", []uint64{1, 2})
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchTransactionsBySourceAndSequences_HappyPath(t *testing.T) {
	sourceDb := newMemoryDB(t, &paymentdb.DBTransaction{})
	// Insert test transactions.
	require.NoError(t, sourceDb.Create(&paymentdb.DBTransaction{
		Hash:          "hash1",
		SourceAddress: "addr",
		Sequence:      10,
		Response:      `{"Fee": "12"}`,
	}).Error)
	require.NoError(t, sourceDb.Create(&paymentdb.DBTransaction{
		Hash:          "hash2",
		SourceAddress: "addr",
		Sequence:      11,
		Response:      `{"Fee": "15"}`,
	}).Error)
	// Insert a transaction for a different address (should not be returned).
	require.NoError(t, sourceDb.Create(&paymentdb.DBTransaction{
		Hash:          "hash3",
		SourceAddress: "other",
		Sequence:      10,
		Response:      `{"Fee": "99"}`,
	}).Error)

	repo := NewDBRepo(sourceDb, nil)
	result, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", []uint64{10, 11})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "hash1", result[10].Hash)
	require.Equal(t, "hash2", result[11].Hash)
}
