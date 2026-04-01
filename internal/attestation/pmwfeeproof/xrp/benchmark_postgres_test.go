//go:build docker_bench

// Run:
//   docker compose -f internal/tests/docker/docker-compose.yaml up -d
//   go test -tags docker_bench -run TestBenchmarkFeeProofPostgres -v ./internal/attestation/pmwfeeproof/xrp/
//   docker compose -f internal/tests/docker/docker-compose.yaml down

package xrpverifier

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/database"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	feeproofdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	benchPostgresURL = "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable"
	benchMysqlURL    = "root:root@tcp(127.0.0.1:3306)/db?parseTime=true"
	benchSender      = "rBenchSender_fee_proof_9999"
	benchBaseNonce   = uint64(900000)
)

func seedBenchData(tb testing.TB, xrpDB, cchainDB *gorm.DB, teeABI abi.ABI, sourceID, opType common.Hash, count uint64) {
	tb.Helper()

	eventHash, err := teeinstruction.TeeInstructionsSentEventSignature(teeABI)
	if err != nil {
		tb.Fatal(err)
	}

	for i := range count {
		nonce := benchBaseNonce + i
		payID, err := instruction.GeneratePayInstructionID(opType, sourceID, benchSender, nonce)
		if err != nil {
			tb.Fatal(err)
		}

		msg := payment.ITeePaymentsPaymentInstructionMessage{
			SenderAddress: benchSender,
			Amount:        big.NewInt(1000),
			MaxFee:        big.NewInt(500),
			TokenId:       []byte{},
			FeeSchedule:   []byte{},
			Nonce:         nonce,
			SubNonce:      nonce,
		}
		eventData := testEncodeEvent(tb, teeABI, op.Pay, msg)

		if err := cchainDB.Create(&database.Log{
			Topic0:          trimHex(eventHash),
			Topic1:          trimHex(common.HexToHash("").Hex()),
			Topic2:          trimHex(payID.Hex()),
			Data:            hex.EncodeToString(eventData),
			Address:         "benchContractAddr",
			TransactionHash: fmt.Sprintf("%064x", nonce),
			LogIndex:        nonce,
			Timestamp:       1700000000,
			BlockNumber:     999999,
		}).Error; err != nil {
			tb.Fatal(err)
		}

		if err := xrpDB.Exec(
			`INSERT INTO transactions (hash, block_number, "timestamp", response, is_native_payment, sequence, ticket_sequence, source_address) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			fmt.Sprintf("benchhash%d", nonce), 999999, 1700000000, `{"Fee":"12"}`, false, nonce, 0, benchSender,
		).Error; err != nil {
			tb.Fatal(err)
		}
	}
}

func cleanupBenchData(xrpDB, cchainDB *gorm.DB) {
	xrpDB.Exec("DELETE FROM transactions WHERE source_address = $1", benchSender)
	cchainDB.Exec("DELETE FROM logs WHERE block_number = ?", 999999)
}

func TestBenchmarkFeeProofPostgres(t *testing.T) {
	origMax := MaxNonceRange
	MaxNonceRange = 1100
	defer func() { MaxNonceRange = origMax }()

	xrpDB, err := gorm.Open(postgres.Open(benchPostgresURL), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("Cannot connect to Postgres: %v", err)
	}
	cchainDB, err := gorm.Open(mysql.Open(benchMysqlURL), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("Cannot connect to MySQL: %v", err)
	}

	// Clean any leftover data from a previous run.
	cleanupBenchData(xrpDB, cchainDB)

	teeABI, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	if err != nil {
		t.Fatal(err)
	}

	sourceID := common.HexToHash("0x1")
	opType := common.HexToHash("0xAA")

	// Seed max data needed (1000 nonces). Smaller counts reuse a subset.
	maxCount := uint64(1000)
	t.Logf("Seeding %d nonces into Postgres + MySQL...", maxCount)
	seedBenchData(t, xrpDB, cchainDB, teeABI, sourceID, opType, maxCount)
	defer cleanupBenchData(xrpDB, cchainDB)

	cfg := &config.PMWFeeProofConfig{
		ParsedTeeInstructionsABI: teeABI,
		EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
	}
	v := &XRPVerifier{
		Repo:   feeproofdb.NewDBRepo(xrpDB, cchainDB),
		Config: cfg,
	}

	counts := []uint64{1, 10, 50, 100, 200, 300, 500, 750, 1000}
	iterations := 50

	type benchResult struct {
		count   uint64
		avgMs   float64
		minMs   float64
		maxMs   float64
		p95Ms   float64
		medMs   float64
	}

	var results []benchResult

	for _, count := range counts {
		req := connector.IPMWFeeProofRequestBody{
			OpType:         opType,
			SenderAddress:  benchSender,
			FromNonce:      benchBaseNonce,
			ToNonce:        benchBaseNonce + count - 1,
			UntilTimestamp: 1800000000,
		}

		// Warmup.
		for range 3 {
			if _, err := v.Verify(context.Background(), req); err != nil {
				t.Fatalf("warmup failed for %d nonces: %v", count, err)
			}
		}

		durations := make([]float64, iterations)
		for i := range iterations {
			start := time.Now()
			_, err := v.Verify(context.Background(), req)
			durations[i] = float64(time.Since(start).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("iteration %d failed for %d nonces: %v", i, count, err)
			}
		}

		sort.Float64s(durations)
		var sum float64
		for _, d := range durations {
			sum += d
		}
		avg := sum / float64(iterations)
		p95Idx := int(float64(len(durations))*0.95) - 1
		if p95Idx < 0 {
			p95Idx = 0
		}

		results = append(results, benchResult{
			count: count,
			avgMs: avg,
			minMs: durations[0],
			maxMs: durations[len(durations)-1],
			p95Ms: durations[p95Idx],
			medMs: durations[len(durations)/2],
		})

		t.Logf("  %4d nonces: avg=%.2fms  min=%.2fms  max=%.2fms  p95=%.2fms",
			count, avg, durations[0], durations[len(durations)-1], durations[p95Idx])
	}

	// Print summary table.
	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════════════════════════╗")
	t.Log("║              PMWFeeProof Benchmark — Postgres + MySQL (Docker)                  ║")
	t.Logf("║  Iterations: %-4d                                                               ║", iterations)
	t.Log("╠══════════╦══════════╦══════════╦══════════╦══════════╦══════════╦═════════════════╣")
	t.Log("║  Nonces  ║ Avg (ms) ║ Med (ms) ║ Min (ms) ║ Max (ms) ║ P95 (ms) ║  Per-nonce (ms) ║")
	t.Log("╠══════════╬══════════╬══════════╬══════════╬══════════╬══════════╬═════════════════╣")
	for _, r := range results {
		perNonce := r.avgMs / float64(r.count)
		t.Logf("║  %6d  ║ %8.2f ║ %8.2f ║ %8.2f ║ %8.2f ║ %8.2f ║  %13.4f  ║",
			r.count, r.avgMs, r.medMs, r.minMs, r.maxMs, r.p95Ms, perNonce)
	}
	t.Log("╚══════════╩══════════╩══════════╩══════════╩══════════╩══════════╩═════════════════╝")

	// Scaling analysis.
	t.Log("")
	if len(results) >= 2 {
		r100 := results[3] // 100 nonces
		r1000 := results[len(results)-1]
		perNonce100 := r100.avgMs / float64(r100.count)
		perNonce1000 := r1000.avgMs / float64(r1000.count)
		ratio := perNonce1000 / perNonce100
		t.Logf("Per-nonce cost at 100: %.4f ms", perNonce100)
		t.Logf("Per-nonce cost at 1000: %.4f ms", perNonce1000)
		t.Logf("Scaling factor: %.2fx", ratio)

		if ratio < 1.5 {
			t.Log("Verdict: LINEAR — MaxNonceRange can safely be increased")
		} else if ratio < 3.0 {
			t.Log("Verdict: MODERATE BEND — current MaxNonceRange is reasonable")
		} else {
			t.Log("Verdict: SUPERLINEAR — consider keeping MaxNonceRange low")
		}
	}
}

// TestBenchmarkFeeProofConcurrent measures how Verify performs under concurrent load.
// Run: docker compose -f internal/tests/docker/docker-compose.yaml up -d
// Then: go test -tags docker_bench -run TestBenchmarkFeeProofConcurrent -v ./internal/attestation/pmwfeeproof/xrp/
func TestBenchmarkFeeProofConcurrent(t *testing.T) {
	origMax := MaxNonceRange
	MaxNonceRange = 1100
	defer func() { MaxNonceRange = origMax }()

	xrpDB, err := gorm.Open(postgres.Open(benchPostgresURL), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("Cannot connect to Postgres: %v", err)
	}
	cchainDB, err := gorm.Open(mysql.Open(benchMysqlURL), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("Cannot connect to MySQL: %v", err)
	}

	// Configure connection pools for concurrent access.
	if sqlDB, err := xrpDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetMaxIdleConns(25)
	}
	if sqlDB, err := cchainDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetMaxIdleConns(25)
	}

	cleanupBenchData(xrpDB, cchainDB)

	teeABI, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	if err != nil {
		t.Fatal(err)
	}

	sourceID := common.HexToHash("0x1")
	opType := common.HexToHash("0xAA")

	maxCount := uint64(1000)
	t.Logf("Seeding %d nonces into Postgres + MySQL...", maxCount)
	seedBenchData(t, xrpDB, cchainDB, teeABI, sourceID, opType, maxCount)
	defer cleanupBenchData(xrpDB, cchainDB)

	cfg := &config.PMWFeeProofConfig{
		ParsedTeeInstructionsABI: teeABI,
		EncodedAndABI:            config.EncodedAndABI{SourceIDPair: config.SourceIDEncodedPair{SourceIDEncoded: sourceID}},
	}
	v := &XRPVerifier{
		Repo:   feeproofdb.NewDBRepo(xrpDB, cchainDB),
		Config: cfg,
	}

	type concScenario struct {
		nonces      uint64
		concurrency int
	}

	scenarios := []concScenario{
		{100, 1},
		{100, 5},
		{100, 10},
		{100, 20},
		{200, 1},
		{200, 5},
		{200, 10},
		{200, 20},
		{300, 1},
		{300, 5},
		{300, 10},
		{300, 20},
		{500, 1},
		{500, 5},
		{500, 10},
		{500, 20},
	}

	iterations := 30

	type concResult struct {
		nonces      uint64
		concurrency int
		avgMs       float64
		medMs       float64
		p95Ms       float64
		maxMs       float64
		throughput  float64 // requests/sec
		errCount    int
	}

	var results []concResult

	for _, sc := range scenarios {
		req := connector.IPMWFeeProofRequestBody{
			OpType:         opType,
			SenderAddress:  benchSender,
			FromNonce:      benchBaseNonce,
			ToNonce:        benchBaseNonce + sc.nonces - 1,
			UntilTimestamp: 1800000000,
		}

		// Warmup.
		for range 3 {
			_, _ = v.Verify(context.Background(), req)
		}

		var allDurations []float64
		var mu sync.Mutex
		var errCount int

		for round := range iterations {
			_ = round
			var wg sync.WaitGroup
			roundDurations := make([]float64, sc.concurrency)
			roundErrors := make([]bool, sc.concurrency)

			for w := range sc.concurrency {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					start := time.Now()
					_, err := v.Verify(context.Background(), req)
					roundDurations[idx] = float64(time.Since(start).Microseconds()) / 1000.0
					if err != nil {
						roundErrors[idx] = true
					}
				}(w)
			}
			wg.Wait()

			mu.Lock()
			for i, d := range roundDurations {
				allDurations = append(allDurations, d)
				if roundErrors[i] {
					errCount++
				}
			}
			mu.Unlock()
		}

		sort.Float64s(allDurations)
		var sum float64
		for _, d := range allDurations {
			sum += d
		}
		n := len(allDurations)
		avg := sum / float64(n)
		p95Idx := int(float64(n)*0.95) - 1
		if p95Idx < 0 {
			p95Idx = 0
		}
		throughput := float64(sc.concurrency) * 1000.0 / avg // req/s based on avg latency

		r := concResult{
			nonces:      sc.nonces,
			concurrency: sc.concurrency,
			avgMs:       avg,
			medMs:       allDurations[n/2],
			p95Ms:       allDurations[p95Idx],
			maxMs:       allDurations[n-1],
			throughput:  throughput,
			errCount:    errCount,
		}
		results = append(results, r)

		t.Logf("  %4d nonces × %2d concurrent: avg=%.1fms  p95=%.1fms  max=%.1fms  throughput=%.1f req/s  errors=%d",
			sc.nonces, sc.concurrency, r.avgMs, r.p95Ms, r.maxMs, r.throughput, r.errCount)
	}

	// Print summary table.
	t.Log("")
	t.Log("╔═══════════════════════════════════════════════════════════════════════════════════════════════╗")
	t.Log("║                PMWFeeProof Concurrent Benchmark — Postgres + MySQL (Docker)                  ║")
	t.Logf("║  Iterations per scenario: %-4d                                                               ║", iterations)
	t.Log("╠═════════╦═══════╦══════════╦══════════╦══════════╦══════════╦════════════╦════════════════════╣")
	t.Log("║ Nonces  ║ Conc  ║ Avg (ms) ║ Med (ms) ║ P95 (ms) ║ Max (ms) ║  Req/s     ║  Errors            ║")
	t.Log("╠═════════╬═══════╬══════════╬══════════╬══════════╬══════════╬════════════╬════════════════════╣")
	for _, r := range results {
		t.Logf("║  %5d  ║  %3d  ║ %8.1f ║ %8.1f ║ %8.1f ║ %8.1f ║ %8.1f   ║  %16d  ║",
			r.nonces, r.concurrency, r.avgMs, r.medMs, r.p95Ms, r.maxMs, r.throughput, r.errCount)
	}
	t.Log("╚═════════╩═══════╩══════════╩══════════╩══════════╩══════════╩════════════╩════════════════════╝")

	// Concurrency impact analysis.
	t.Log("")
	for _, nonces := range []uint64{100, 200, 300, 500} {
		var baseline, max20 concResult
		for _, r := range results {
			if r.nonces == nonces && r.concurrency == 1 {
				baseline = r
			}
			if r.nonces == nonces && r.concurrency == 20 {
				max20 = r
			}
		}
		if baseline.avgMs > 0 && max20.avgMs > 0 {
			slowdown := max20.avgMs / baseline.avgMs
			t.Logf("%d nonces: 1→20 concurrent slowdown: %.2fx (%.1fms → %.1fms avg, p95: %.1fms → %.1fms)",
				nonces, slowdown, baseline.avgMs, max20.avgMs, baseline.p95Ms, max20.p95Ms)
		}
	}
}
