package db

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testContractAddress is the canonical contract address used in tests.
var testContractAddress = common.HexToAddress("0x00000000000000000000000000000000000000C1")

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
	repo := NewDBRepo(nil, newClosedDB(t), testContractAddress)
	ids := []common.Hash{common.HexToHash("0x1")}
	_, err := repo.FetchInstructionLogs(context.Background(), "0xabc", ids)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch instruction logs")
}

func TestFetchInstructionLogs_EmptyIDs(t *testing.T) {
	repo := NewDBRepo(nil, nil, testContractAddress)
	result, err := repo.FetchInstructionLogs(context.Background(), "0xabc", nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchInstructionLogs_NoResults(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	ids := []common.Hash{common.HexToHash("0x1")}
	result, err := repo.FetchInstructionLogs(context.Background(), "0xabc", ids)
	require.NoError(t, err)
	require.Empty(t, result)
}

// TestFetchInstructionLogs_WrongContractAddress verifies that rows whose emitter address
// does not match the configured contract are filtered out even when topics match.
func TestFetchInstructionLogs_WrongContractAddress(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")

	require.NoError(t, cchainDb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Address:         "00000000000000000000000000000000000000de", // wrong address
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     1,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	result, err := repo.FetchInstructionLogs(context.Background(), eventHash, []common.Hash{id})
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchInstructionLogs_DuplicateRowsAreRejected(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")
	contractAddr := strings.TrimPrefix(strings.ToLower(testContractAddress.Hex()), "0x")

	for i := range 2 {
		require.NoError(t, cchainDb.Create(&database.Log{
			Topic0:          eventHash,
			Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
			Topic2:          topic2,
			Address:         contractAddr,
			TransactionHash: strings.Repeat("0", 63) + string(rune('0'+i)),
			LogIndex:        uint64(i),
			BlockNumber:     uint64(10 + i),
			Timestamp:       1700000000,
		}).Error)
	}

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	_, err := repo.FetchInstructionLogs(context.Background(), eventHash, []common.Hash{id})
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "duplicate logs for instruction")
}

func TestFetchInstructionLogs_CorrectAddressReturnsLogs(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")
	contractAddr := strings.TrimPrefix(strings.ToLower(testContractAddress.Hex()), "0x")

	require.NoError(t, cchainDb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Address:         contractAddr,
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     1,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	result, err := repo.FetchInstructionLogs(context.Background(), eventHash, []common.Hash{id})
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestFetchInstructionLog_CorrectAddressReturnsLog(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")
	contractAddr := strings.TrimPrefix(strings.ToLower(testContractAddress.Hex()), "0x")

	require.NoError(t, cchainDb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Address:         contractAddr,
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     1,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	result, err := repo.FetchInstructionLog(context.Background(), eventHash, id)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Log)
	require.Equal(t, uint64(1700000000), result.BlockTimestamp)
}

func TestFetchInstructionLog_DatabaseError(t *testing.T) {
	repo := NewDBRepo(nil, newClosedDB(t), testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), "0xabc", common.HexToHash("0x1"))
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch log for instruction")
}

func TestFetchInstructionLog_RecordNotFound(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), "0xabc", common.HexToHash("0x1"))
	require.ErrorIs(t, err, paymentdb.ErrRecordNotFound)
}

// TestFetchInstructionLog_WrongContractAddress verifies the single-log path rejects
// events from a non-canonical contract address.
func TestFetchInstructionLog_WrongContractAddress(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")

	require.NoError(t, cchainDb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Address:         "00000000000000000000000000000000000000de", // wrong address
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     1,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), eventHash, id)
	require.ErrorIs(t, err, paymentdb.ErrRecordNotFound)
}

func TestFetchInstructionLogs_ConvertError(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")
	contractAddr := strings.TrimPrefix(strings.ToLower(testContractAddress.Hex()), "0x")

	require.NoError(t, cchainDb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Data:            "not-valid-hex",
		Address:         contractAddr,
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     1,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	_, err := repo.FetchInstructionLogs(context.Background(), eventHash, []common.Hash{id})
	require.ErrorContains(t, err, "cannot convert log")
}

func TestFetchInstructionLog_ConvertError(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")
	contractAddr := strings.TrimPrefix(strings.ToLower(testContractAddress.Hex()), "0x")

	require.NoError(t, cchainDb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Data:            "not-valid-hex",
		Address:         contractAddr,
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     1,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), eventHash, id)
	require.Error(t, err)
}

func TestFetchInstructionLog_DuplicateRowsAreRejected(t *testing.T) {
	cchainDb := newMemoryDB(t, &database.Log{})
	id := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(id.Hex(), "0x")
	contractAddr := strings.TrimPrefix(strings.ToLower(testContractAddress.Hex()), "0x")

	for i := range 2 {
		require.NoError(t, cchainDb.Create(&database.Log{
			Topic0:          eventHash,
			Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
			Topic2:          topic2,
			Address:         contractAddr,
			TransactionHash: strings.Repeat("0", 63) + string(rune('0'+i)),
			LogIndex:        uint64(i),
			BlockNumber:     uint64(10 + i),
			Timestamp:       1700000000,
		}).Error)
	}

	repo := NewDBRepo(nil, cchainDb, testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), eventHash, id)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "duplicate logs for instruction")
}

func TestFetchTransactionsBySourceAndSequences_DatabaseError(t *testing.T) {
	repo := NewDBRepo(newClosedDB(t), nil, testContractAddress)
	_, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", []uint64{1, 2})
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch transactions for source")
}

func TestFetchTransactionsBySourceAndSequences_EmptyNonces(t *testing.T) {
	repo := NewDBRepo(nil, nil, testContractAddress)
	result, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchTransactionsBySourceAndSequences_NoResults(t *testing.T) {
	sourceDb := newMemoryDB(t, &paymentdb.DBTransaction{})
	repo := NewDBRepo(sourceDb, nil, testContractAddress)
	result, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", []uint64{1, 2})
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFetchTransactionsBySourceAndSequences_DuplicateRowsAreRejected(t *testing.T) {
	sourceDb := newMemoryDB(t, &paymentdb.DBTransaction{})
	for i := range 2 {
		require.NoError(t, sourceDb.Create(&paymentdb.DBTransaction{
			Hash:          fmt.Sprintf("hash%d", i),
			SourceAddress: "addr",
			Sequence:      10,
			Response:      `{"Fee": "12"}`,
		}).Error)
	}

	repo := NewDBRepo(sourceDb, nil, testContractAddress)
	_, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", []uint64{10})
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
	require.ErrorContains(t, err, "duplicate transactions for source")
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

	repo := NewDBRepo(sourceDb, nil, testContractAddress)
	result, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "addr", []uint64{10, 11})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "hash1", result[10].Hash)
	require.Equal(t, "hash2", result[11].Hash)
}
