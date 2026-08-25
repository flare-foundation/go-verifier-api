package db

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testContractAddress is the canonical contract address used in tests; the indexer
// stores this as lowercase hex without the 0x prefix.
var testContractAddress = common.HexToAddress("0x00000000000000000000000000000000000000C1")

// testContractAddressStored is the value of database.Log.Address that matches testContractAddress.
const testContractAddressStored = "00000000000000000000000000000000000000c1"

// testEventHash is the canonical Topic0 (event signature) used across these tests.
const testEventHash = "abcd"

func newClosedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, CloseDB(db))
	return db
}

func TestFetchInstructionLog_DatabaseError(t *testing.T) {
	repo := NewDBRepo(nil, newClosedDB(t), testContractAddress)
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
	repo := NewDBRepo(nil, newMemoryDBWithLogs(t), testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), "abc", common.HexToHash("0x1"))
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestFetchInstructionLog_DuplicateRowsAreRejected(t *testing.T) {
	cdb := newMemoryDBWithLogs(t)
	instructionID := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(instructionID.Hex(), "0x")

	// Insert two logs with identical (address, topic0, topic1, topic2).
	for i := range 2 {
		require.NoError(t, cdb.Create(&database.Log{
			Topic0:          eventHash,
			Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
			Topic2:          topic2,
			Data:            "",
			Address:         testContractAddressStored,
			TransactionHash: strings.Repeat("0", 63) + string(rune('0'+i)),
			LogIndex:        uint64(i),
			BlockNumber:     uint64(10 + i),
			Timestamp:       1700000000,
		}).Error)
	}

	repo := NewDBRepo(nil, cdb, testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), eventHash, instructionID)
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "duplicate logs for instruction")
}

// TestFetchInstructionLog_WrongContractAddress verifies that a log with matching topics
// but emitted by a different contract is ignored, producing ErrRecordNotFound.
func TestFetchInstructionLog_WrongContractAddress(t *testing.T) {
	cdb := newMemoryDBWithLogs(t)
	instructionID := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(instructionID.Hex(), "0x")

	// Insert a log with matching topics but a DIFFERENT emitter address.
	require.NoError(t, cdb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Data:            "",
		Address:         "00000000000000000000000000000000000000de", // not testContractAddressStored
		TransactionHash: strings.Repeat("0", 63) + "0",
		LogIndex:        0,
		BlockNumber:     10,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cdb, testContractAddress)
	_, err := repo.FetchInstructionLog(context.Background(), eventHash, instructionID)
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestFetchInstructionLog_CorrectAddressReturnsLog(t *testing.T) {
	cdb := newMemoryDBWithLogs(t)
	instructionID := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	topic2 := strings.TrimPrefix(instructionID.Hex(), "0x")

	require.NoError(t, cdb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          "0000000000000000000000000000000000000000000000000000000000000000",
		Topic2:          topic2,
		Data:            "",
		Address:         testContractAddressStored,
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     10,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cdb, testContractAddress)
	log, err := repo.FetchInstructionLog(context.Background(), eventHash, instructionID)
	require.NoError(t, err)
	require.NotNil(t, log)
}

func TestNormalizeAddress(t *testing.T) {
	// Mixed-case checksummed address should normalize to lowercase without 0x prefix.
	addr := common.HexToAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B")
	got := normalizeAddress(addr)
	require.Equal(t, "ab5801a7d398351b8be11c439e05c5b3259aec9b", got)
}

func TestFetchTransactionBySourceAndSequence_DatabaseError(t *testing.T) {
	repo := NewDBRepo(newClosedDB(t), nil, testContractAddress)
	_, err := repo.FetchTransactionBySourceAndSequence(context.Background(), ChainQuery{SourceAddress: "addr", Nonce: 1})
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "cannot fetch transaction for source")
}

func TestFetchTransactionBySourceAndSequence_NotFound(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DBTransaction{}))

	repo := NewDBRepo(db, nil, testContractAddress)
	_, err = repo.FetchTransactionBySourceAndSequence(context.Background(), ChainQuery{SourceAddress: "addr", Nonce: 1})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestFetchTransactionBySourceAndSequence_Success(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DBTransaction{}))
	require.NoError(t, db.Create(&DBTransaction{
		Hash:          "hash1",
		SourceAddress: "rSender",
		Sequence:      42,
		Response:      `{"Fee":"12"}`,
	}).Error)

	repo := NewDBRepo(db, nil, testContractAddress)
	tx, err := repo.FetchTransactionBySourceAndSequence(context.Background(), ChainQuery{SourceAddress: "rSender", Nonce: 42})
	require.NoError(t, err)
	require.Equal(t, "hash1", tx.Hash)
	require.Equal(t, uint64(42), tx.Sequence)
}

func TestFetchTransactionBySourceAndSequence_DuplicateRowsAreRejected(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DBTransaction{}))
	// Two rows for the same (source_address, sequence) but different Hash — an
	// honest XRPL indexer can't produce this (sequence is consumed once), so it
	// must fail closed rather than silently pick one.
	for _, h := range []string{"hashA", "hashB"} {
		require.NoError(t, db.Create(&DBTransaction{
			Hash:          h,
			SourceAddress: "rSender",
			Sequence:      42,
			Response:      `{"Fee":"12"}`,
		}).Error)
	}

	repo := NewDBRepo(db, nil, testContractAddress)
	_, err = repo.FetchTransactionBySourceAndSequence(context.Background(), ChainQuery{SourceAddress: "rSender", Nonce: 42})
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "duplicate transactions")
}

// TestFetchLogsByInstructionTopic1_RejectsOversizedData: a row whose data exceeds
// the per-row byte cap is refused at the DB before materializing, so a hostile set
// of huge rows cannot be loaded into memory and decoded.
func TestFetchLogsByInstructionTopic1_RejectsOversizedData(t *testing.T) {
	cdb := newMemoryDBWithLogs(t)
	instructionID := common.HexToHash("0xdeadbeef")
	eventHash := "abcd"
	require.NoError(t, cdb.Create(&database.Log{
		Topic0:          eventHash,
		Topic1:          strings.TrimPrefix(instructionID.Hex(), "0x"),
		Data:            strings.Repeat("a", maxEventDataHexLen+2), // over the per-row byte cap
		Address:         testContractAddressStored,
		TransactionHash: strings.Repeat("0", 64),
		LogIndex:        0,
		BlockNumber:     10,
		Timestamp:       1700000000,
	}).Error)

	repo := NewDBRepo(nil, cdb, testContractAddress)
	_, err := repo.FetchLogsByInstructionTopic1(context.Background(), eventHash, instructionID)
	require.ErrorIs(t, err, ErrDatabase)
	require.ErrorContains(t, err, "exceed")
}

// topic1Log builds a PaymentBatched-shaped row for FetchLogsByInstructionTopic1
// (topic1 = instruction id), with a unique transaction hash per index.
func topic1Log(instructionID common.Hash, data, addr string, i int) database.Log {
	return database.Log{
		Topic0:          testEventHash,
		Topic1:          strings.TrimPrefix(instructionID.Hex(), "0x"),
		Topic2:          strings.Repeat("0", 64),
		Data:            data,
		Address:         addr,
		TransactionHash: fmt.Sprintf("%064x", i),
		LogIndex:        uint64(i),
		BlockNumber:     uint64(10 + i),
		Timestamp:       1700000000,
	}
}

func TestFetchLogsByInstructionTopic1_RowCapBoundary(t *testing.T) {
	const eventHash = "abcd"
	instructionID := common.HexToHash("0xdeadbeef")
	insertN := func(t *testing.T, n int) *gorm.DB {
		t.Helper()
		db := newMemoryDBWithLogs(t)
		rows := make([]database.Log, n)
		for i := range n {
			rows[i] = topic1Log(instructionID, "", testContractAddressStored, i)
		}
		require.NoError(t, db.CreateInBatches(rows, 1000).Error)
		return db
	}
	t.Run("exactly max accepted", func(t *testing.T) {
		repo := NewDBRepo(nil, insertN(t, maxInstructionLogs), testContractAddress)
		logs, err := repo.FetchLogsByInstructionTopic1(context.Background(), eventHash, instructionID)
		require.NoError(t, err)
		require.Len(t, logs, maxInstructionLogs)
	})
	t.Run("one over max rejected", func(t *testing.T) {
		repo := NewDBRepo(nil, insertN(t, maxInstructionLogs+1), testContractAddress)
		_, err := repo.FetchLogsByInstructionTopic1(context.Background(), eventHash, instructionID)
		require.ErrorIs(t, err, ErrDatabase)
	})
}

func TestFetchLogsByInstructionTopic1_DataSizeBoundary(t *testing.T) {
	const eventHash = "abcd"
	instructionID := common.HexToHash("0xdeadbeef")
	t.Run("at cap accepted", func(t *testing.T) {
		db := newMemoryDBWithLogs(t)
		require.NoError(t, db.Create(&[]database.Log{topic1Log(instructionID, strings.Repeat("a", maxEventDataHexLen), testContractAddressStored, 0)}).Error)
		repo := NewDBRepo(nil, db, testContractAddress)
		logs, err := repo.FetchLogsByInstructionTopic1(context.Background(), eventHash, instructionID)
		require.NoError(t, err)
		require.Len(t, logs, 1)
	})
	t.Run("one over cap rejected", func(t *testing.T) {
		db := newMemoryDBWithLogs(t)
		require.NoError(t, db.Create(&[]database.Log{topic1Log(instructionID, strings.Repeat("a", maxEventDataHexLen+2), testContractAddressStored, 0)}).Error)
		repo := NewDBRepo(nil, db, testContractAddress)
		_, err := repo.FetchLogsByInstructionTopic1(context.Background(), eventHash, instructionID)
		require.ErrorIs(t, err, ErrDatabase)
		require.ErrorContains(t, err, "exceed")
	})
}

// TestFetchLogsByInstructionTopic1_OversizedOtherContractDoesNotPoison: the size
// check is scoped to the query's own filter, so a huge row for a different
// contract must not reject our (well-formed) result.
func TestFetchLogsByInstructionTopic1_OversizedOtherContractDoesNotPoison(t *testing.T) {
	const eventHash = "abcd"
	instructionID := common.HexToHash("0xdeadbeef")
	db := newMemoryDBWithLogs(t)
	other := topic1Log(instructionID, strings.Repeat("a", maxEventDataHexLen+2), "ffffffffffffffffffffffffffffffffffffffff", 0)
	ours := topic1Log(instructionID, "", testContractAddressStored, 1)
	require.NoError(t, db.Create(&[]database.Log{other, ours}).Error)

	repo := NewDBRepo(nil, db, testContractAddress)
	logs, err := repo.FetchLogsByInstructionTopic1(context.Background(), eventHash, instructionID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
}

func TestFetchLogsByInstructionTopic1_ClosedDB(t *testing.T) {
	repo := NewDBRepo(nil, newClosedDB(t), testContractAddress)
	_, err := repo.FetchLogsByInstructionTopic1(context.Background(), "abcd", common.HexToHash("0x1"))
	require.ErrorIs(t, err, ErrDatabase)
}

func TestFetchLogsByInstructionTopic1_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewDBRepo(nil, newMemoryDBWithLogs(t), testContractAddress)
	_, err := repo.FetchLogsByInstructionTopic1(ctx, "abcd", common.HexToHash("0x1"))
	require.ErrorIs(t, err, ErrDatabase)
}
