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
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSharedMemoryDB(t *testing.T, name string, models ...any) *gorm.DB {
	t.Helper()
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

// TestLoadFeeProofDBBatchFetch simulates concurrent batch fetches
// of transactions by source and nonce range.
func TestLoadFeeProofDBBatchFetch(t *testing.T) {
	xrpDB := newSharedMemoryDB(t, "feeproof_xrp", &paymentdb.DBTransaction{})

	// Seed 20 transactions.
	for i := 0; i < 20; i++ {
		tx := paymentdb.DBTransaction{
			Hash:          fmt.Sprintf("hash%d", i),
			BlockNumber:   100,
			Timestamp:     1700000000,
			Response:      fmt.Sprintf(`{"Fee":"%d"}`, 10+i),
			SourceAddress: "rSender",
			Sequence:      uint64(100 + i),
		}
		if err := xrpDB.Create(&tx).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewDBRepo(xrpDB, nil, testContractAddress)

	const (
		concurrency = 100
		rounds      = 20
	)

	nonces := make([]uint64, 10)
	for i := range nonces {
		nonces[i] = uint64(100 + i)
	}

	var allLatencies []time.Duration
	var mu sync.Mutex

	for round := 0; round < rounds; round++ {
		type callResult struct {
			txMap map[uint64]paymentdb.DBTransaction
			err   error
		}
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				txMap, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "rSender", nonces)
				elapsed := time.Since(start)
				mu.Lock()
				allLatencies = append(allLatencies, elapsed)
				mu.Unlock()
				results[idx] = callResult{txMap: txMap, err: err}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			if r.err != nil {
				t.Fatalf("round %d, caller %d: unexpected error: %v", round, i, r.err)
			}
			if len(r.txMap) != len(nonces) {
				t.Fatalf("round %d, caller %d: expected %d txs, got %d", round, i, len(nonces), len(r.txMap))
			}
		}
	}

	sort.Slice(allLatencies, func(i, j int) bool { return allLatencies[i] < allLatencies[j] })
	n := len(allLatencies)
	t.Logf("FeeProof DB batch fetch: n=%d, p50=%v, p95=%v, p99=%v",
		n, allLatencies[n*50/100], allLatencies[n*95/100], allLatencies[n*99/100])
}

// TestLoadFeeProofDBBatchFetchInstructionLogs simulates concurrent batch fetches
// of instruction logs from the C-Chain DB.
func TestLoadFeeProofDBBatchFetchInstructionLogs(t *testing.T) {
	cChainDB := newSharedMemoryDB(t, "feeproof_cchain", &database.Log{})

	ids := make([]common.Hash, 10)
	for i := 0; i < 10; i++ {
		id := common.HexToHash(fmt.Sprintf("0x%064x", i+1))
		ids[i] = id
		log := database.Log{
			Topic0:          "eventHash",
			Topic1:          common.HexToHash("").Hex()[2:],
			Topic2:          id.Hex()[2:],
			Data:            "deadbeef",
			Address:         testContractAddressStored,
			TransactionHash: fmt.Sprintf("%064x", i+1),
			LogIndex:        uint64(i),
			Timestamp:       1700000000,
			BlockNumber:     100,
		}
		if err := cChainDB.Create(&log).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewDBRepo(nil, cChainDB, testContractAddress)

	const (
		concurrency = 100
		rounds      = 20
	)

	var allLatencies []time.Duration
	var mu sync.Mutex

	for round := 0; round < rounds; round++ {
		type callResult struct {
			logs map[common.Hash]any
			err  error
		}
		results := make([]callResult, concurrency)
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				logs, err := repo.FetchInstructionLogs(context.Background(), "eventHash", ids)
				elapsed := time.Since(start)
				mu.Lock()
				allLatencies = append(allLatencies, elapsed)
				mu.Unlock()
				_ = logs
				results[idx] = callResult{err: err}
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
	t.Logf("FeeProof DB instruction log batch fetch: n=%d, p50=%v, p95=%v, p99=%v",
		n, allLatencies[n*50/100], allLatencies[n*95/100], allLatencies[n*99/100])
}

// TestLoadFeeProofDBClosedConnection verifies consistent error behavior
// under concurrent requests when DB is unavailable.
func TestLoadFeeProofDBClosedConnection(t *testing.T) {
	xrpDB := newSharedMemoryDB(t, "feeproof_closed", &paymentdb.DBTransaction{})
	sqlDB, err := xrpDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.Close()

	repo := NewDBRepo(xrpDB, nil, testContractAddress)

	const concurrency = 100
	type callResult struct {
		err error
	}
	results := make([]callResult, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := repo.FetchTransactionsBySourceAndSequences(context.Background(), "rSender", []uint64{1, 2, 3})
			results[idx] = callResult{err: err}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.err == nil {
			t.Fatalf("caller %d: expected error for closed DB", i)
		}
		if !errors.Is(r.err, paymentdb.ErrDatabase) {
			t.Fatalf("caller %d: expected ErrDatabase, got: %v", i, r.err)
		}
	}
}
