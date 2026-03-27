//go:build load

package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSharedMemoryDB(t *testing.T, name string, models ...any) *gorm.DB {
	t.Helper()
	// Use shared cache so all GORM pool connections see the same data.
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// TestLoadPaymentStatusDBConcurrentReads simulates concurrent DB reads
// from multiple data providers querying the same transaction.
func TestLoadPaymentStatusDBConcurrentReads(t *testing.T) {
	xrpDB := newSharedMemoryDB(t, "load_xrp", &DBTransaction{})
	cChainDB := newSharedMemoryDB(t, "load_cchain", &database.Log{})

	// Seed test data.
	tx := DBTransaction{
		Hash:          "abc123",
		BlockNumber:   100,
		Timestamp:     1700000000,
		Response:      `{"Fee":"10"}`,
		SourceAddress: "rSender",
		Sequence:      42,
	}
	if err := xrpDB.Create(&tx).Error; err != nil {
		t.Fatal(err)
	}

	log := database.Log{
		Topic0:  "eventHash",
		Topic1:  common.HexToHash("").Hex()[2:],
		Topic2:  common.HexToHash("0x1").Hex()[2:],
		Data:    "deadbeef",
		Address: "contractAddr",
	}
	if err := cChainDB.Create(&log).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewDBRepo(xrpDB, cChainDB)

	const (
		concurrency = 12
		rounds      = 30
	)

	type callResult struct {
		err     error
		elapsed time.Duration
	}

	var allLatencies []time.Duration
	var mu sync.Mutex

	for round := 0; round < rounds; round++ {
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				_, err := repo.GetTransactionBySourceAndSequence(context.Background(),
					ChainQuery{SourceAddress: "rSender", Nonce: 42})
				elapsed := time.Since(start)
				results[idx] = callResult{err: err, elapsed: elapsed}
				mu.Lock()
				allLatencies = append(allLatencies, elapsed)
				mu.Unlock()
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			if r.err != nil {
				t.Fatalf("round %d, caller %d: unexpected error: %v", round, i, r.err)
			}
		}
	}

	sort.Slice(allLatencies, func(i, j int) bool { return allLatencies[i] < allLatencies[j] })
	n := len(allLatencies)
	t.Logf("PaymentStatus DB concurrent reads: n=%d, p50=%v, p95=%v, p99=%v",
		n, allLatencies[n*50/100], allLatencies[n*95/100], allLatencies[n*99/100])
}

// TestLoadPaymentStatusDBMissingRecord verifies consistent error behavior
// under concurrent requests for a non-existent record.
func TestLoadPaymentStatusDBMissingRecord(t *testing.T) {
	xrpDB := newSharedMemoryDB(t, "load_missing", &DBTransaction{})
	repo := NewDBRepo(xrpDB, nil)

	const concurrency = 12
	type callResult struct {
		err error
	}

	results := make([]callResult, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := repo.GetTransactionBySourceAndSequence(context.Background(),
				ChainQuery{SourceAddress: "nonexistent", Nonce: 999})
			results[idx] = callResult{err: err}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.err == nil {
			t.Fatalf("caller %d: expected error for missing record", i)
		}
		if !isRecordNotFound(r.err) {
			t.Fatalf("caller %d: expected ErrRecordNotFound, got: %v", i, r.err)
		}
	}
}

// TestLoadPaymentStatusDBClosedConnection verifies consistent error behavior
// when the DB connection is unavailable (simulates infrastructure failure).
func TestLoadPaymentStatusDBClosedConnection(t *testing.T) {
	closedDB := newClosedDB(t)
	repo := NewDBRepo(closedDB, nil)

	const concurrency = 12
	type callResult struct {
		err error
	}

	results := make([]callResult, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := repo.GetTransactionBySourceAndSequence(context.Background(),
				ChainQuery{SourceAddress: "addr", Nonce: 1})
			results[idx] = callResult{err: err}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.err == nil {
			t.Fatalf("caller %d: expected error for closed DB", i)
		}
		if !isDBError(r.err) {
			t.Fatalf("caller %d: expected ErrDatabase, got: %v", i, r.err)
		}
	}
}

func isRecordNotFound(err error) bool {
	return err != nil && errors.Is(err, ErrRecordNotFound)
}

func isDBError(err error) bool {
	return err != nil && errors.Is(err, ErrDatabase)
}