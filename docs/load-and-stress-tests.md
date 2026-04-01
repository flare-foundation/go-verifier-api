# Load and Stress Test Report

## Overview

Package-level and component-level tests that verify the verifier's behavior under concurrent production-like traffic and beyond-normal conditions. These are not full end-to-end server benchmarks — they test internal components with mock dependencies. They are gated behind build tags and do not run during normal `go test` or `gencover.sh`.

- **Load tests** (`-tags load`): simulate expected production traffic (100 concurrent requests)
- **Stress tests** (`-tags stress`): push beyond expected limits (up to 1000 concurrent, sustained 30s runs)

## Test Suite

### Load Tests

| Package | Test | What it verifies |
|---|---|---|
| TEE CRL cache | `TestLoadCRLCacheConcurrentBurst` | 100 concurrent requests × 50 rounds, singleflight dedup, result consistency |
| TEE CRL cache | `TestLoadCRLCacheFailedFetchNeverCached` | Failed fetch shared via singleflight, retry succeeds in wave 2 |
| TEE CRL cache | `TestLoadCRLCacheWrongIssuerNeverShared` | Wrong-issuer CRL rejected for all 100 callers, never cached |
| TEE CRL cache | `TestLoadCRLCacheMixedURLs` | Different URLs don't cross-contaminate under concurrency |
| TEE poller | `TestLoadPollerSingleCycleCompletion` | Cycle time for 10/50/100/200/500 TEEs |
| TEE poller | `TestLoadPollerSlowTEEsDoNotStallCycle` | 100 TEEs (25 slow @ 2s) — slow TEEs don't block healthy ones |
| TEE poller | `TestLoadPollerRepeatedCyclesStability` | 20 cycles × 100 TEEs — no goroutine leaks, samples capped |
| Multisig | `TestLoadMultisigConcurrentVerify` | 100 concurrent verify calls against mock RPC |
| Multisig | `TestLoadMultisigSlowUpstream` | 3s slow RPC completes within 4s client timeout |
| PaymentStatus DB | `TestLoadPaymentStatusDBConcurrentReads` | 100 concurrent DB reads |
| PaymentStatus DB | `TestLoadPaymentStatusDBMissingRecord` | Consistent error under concurrency |
| PaymentStatus DB | `TestLoadPaymentStatusDBClosedConnection` | Consistent DB error under concurrency |
| PaymentStatus verifier | `TestLoadPaymentStatusConcurrentVerify` | 100 concurrent full verify flows (ABI decode + DB + response) |
| FeeProof DB | `TestLoadFeeProofDBBatchFetch` | 100 concurrent batch transaction fetches |
| FeeProof DB | `TestLoadFeeProofDBBatchFetchInstructionLogs` | 100 concurrent batch log fetches |
| FeeProof DB | `TestLoadFeeProofDBClosedConnection` | Consistent DB error under concurrency |
| FeeProof verifier | `TestLoadFeeProofConcurrentVerify` | 100 concurrent full verify flows (batch events + tx fees) |

### Stress Tests

| Package | Test | What it verifies |
|---|---|---|
| TEE CRL cache | `TestStressCRLCacheConcurrencyRamp` | 100/250/500/1000 concurrent unique URLs |
| TEE CRL cache | `TestStressCRLCacheSlowUpstream` | 100 healthy + 50 slow independent URLs — isolation |
| TEE CRL cache | `TestStressCRLCacheSustained` | 30s sustained, 50 workers, 200 rotating URLs — heap/goroutine stability |
| TEE poller | `TestStressPollerConcurrencyRamp` | 100/250/500/1000 TEEs per cycle |
| TEE poller | `TestStressPollerSlowUpstreamIsolation` | 200 TEEs (100 slow @ 3s) — worker pool saturation |
| TEE poller | `TestStressPollerSustained` | 50 cycles × 100 TEEs — heap/goroutine stability |

## Results

### CRL Cache

| Scenario | p50 | p95 | p99 | Errors | Notes |
|---|---|---|---|---|---|
| 100 concurrent (unique URLs) | 16ms | 17ms | 18ms | 0 | |
| 250 concurrent | 13ms | 14ms | 15ms | 0 | |
| 500 concurrent | 14ms | 20ms | 20ms | 0 | |
| 1000 concurrent | 18ms | 21ms | 22ms | 0 | Remains stable up to 1000 concurrent unique URLs |
| Slow upstream (100 healthy + 50 slow) | 9ms (healthy) | 11ms | 11ms | 0 healthy | Slow doesn't affect healthy |
| 30s sustained (50 workers) | - | - | - | 0 | 13.6k ops/sec, 0 heap growth, 0 goroutine growth |

### TEE Poller

| Scenario | Cycle Time | Status |
|---|---|---|
| 10 TEEs | 12ms | OK |
| 50 TEEs | 57ms | OK |
| 100 TEEs | 111ms | OK |
| 200 TEEs | 233ms | OK |
| 500 TEEs | 593ms | OK |
| 1000 TEEs | 1.2s | OK — well under 1-minute interval |
| 100 TEEs (25 slow @ 2s) | 6s | OK |
| 200 TEEs (100 slow @ 3s) | 30s | Worker pool saturated (10 workers × 10 batches × 3s) |
| **500 TEEs (200 slow @ 4s)** | **80s** | **Exceeds 1-minute polling interval** |
| 50 cycles × 100 TEEs sustained | 58-115ms/cycle | 0 goroutine growth, 0 heap growth |

### PMW Attestation Types

| Scenario | p50 | p95 | p99 | Notes |
|---|---|---|---|---|
| Multisig: 100 concurrent verify | 5.9ms | 14ms | 42ms | Mock RPC |
| Multisig: 6 concurrent + 3s slow RPC | 3s | 3s | 3s | All complete within timeout |
| PaymentStatus DB: 100 concurrent reads | 36µs | 3.9ms | 5.8ms | SQLite in-memory |
| PaymentStatus verifier: 100 concurrent | 6.7ms | 17ms | 24ms | Full ABI decode flow |
| FeeProof DB: 100 concurrent batch fetch | 3.4ms | 9.5ms | 29ms | 10-nonce batch |
| FeeProof verifier: 100 concurrent | 30ms | 64ms | 84ms | 5-nonce range |

## Key Findings

1. **CRL cache scales well** — 1000 concurrent unique URLs at p99=22ms with zero errors. Singleflight prevents duplicate fetches for the same URL. Slow upstreams don't cause collateral slowdown.

2. **Poller has a scaling limit with slow TEEs** — with 10 workers, many slow TEEs saturate the pool. 500 TEEs with 200 slow @ 4s exceeds the 1-minute interval. This is mitigated by extension filtering (`MAX_POLLED_TEES`) which caps the total polled machines.

3. **All verifier paths handle concurrency correctly** — no races, panics, or inconsistent results under 100 concurrent requests across all attestation types.

4. **No meaningful memory or goroutine growth observed** — sustained 30s CRL cache run and 50-cycle poller run showed no heap or goroutine growth in testing.

## Thresholds

| Metric | Threshold | Purpose |
|---|---|---|
| CRL healthy p95 | < 500ms | Slow upstream isolation |
| Goroutine growth | < 10 | Leak detection |
| Heap growth | < 50MB | Memory leak detection |
| Error rate (healthy paths) | 0 | Correctness under load |
| Poller cycle (all healthy) | < 1 minute | Must complete within polling interval |

## PMWFeeProof Benchmark (Postgres + MySQL)

Docker-dependent benchmarks that measure `Verify` latency and scaling with real Postgres (XRP indexer) and MySQL (C-chain indexer) databases. Gated behind the `docker_bench` build tag.

### Sequential (single client)

50 iterations per nonce count. Data seeded into real DB tables, cleaned up after.

| Nonces | Avg (ms) | Med (ms) | Min (ms) | Max (ms) | P95 (ms) | Per-nonce (ms) |
|--------|----------|----------|----------|----------|----------|----------------|
| 1 | 1.35 | 1.34 | 0.97 | 2.27 | 1.55 | 1.346 |
| 10 | 8.15 | 8.11 | 6.84 | 10.23 | 9.44 | 0.815 |
| 50 | 33.11 | 33.00 | 28.84 | 48.07 | 37.12 | 0.662 |
| 100 | 67.62 | 65.36 | 56.58 | 150.37 | 76.50 | 0.676 |
| 200 | 135.06 | 126.05 | 106.46 | 274.26 | 168.45 | 0.675 |
| 300 | 193.92 | 184.52 | 165.90 | 277.25 | 237.40 | 0.646 |
| 500 | 310.07 | 305.32 | 289.04 | 372.49 | 332.30 | 0.620 |
| 750 | 472.65 | 476.75 | 435.15 | 538.02 | 506.17 | 0.630 |
| 1000 | 632.31 | 623.40 | 577.47 | 815.96 | 711.05 | 0.632 |

Per-nonce cost at 100: 0.676 ms. Per-nonce cost at 1000: 0.632 ms. **Scaling factor: 0.94x — perfectly linear.**

### Concurrent (multiple clients)

30 iterations per scenario. Connection pool: 50 max open, 25 idle.

| Nonces | Conc | Avg (ms) | Med (ms) | P95 (ms) | Max (ms) | Req/s |
|--------|------|----------|----------|----------|----------|-------|
| 100 | 1 | 62.0 | 61.0 | 65.5 | 75.7 | 16.1 |
| 100 | 5 | 122.6 | 122.6 | 134.6 | 167.2 | 40.8 |
| 100 | 10 | 160.0 | 158.3 | 180.4 | 197.5 | 62.5 |
| 100 | 20 | 212.5 | 206.5 | 249.7 | 256.7 | 94.1 |
| 200 | 1 | 121.5 | 120.9 | 129.1 | 139.3 | 8.2 |
| 200 | 5 | 232.7 | 230.1 | 249.2 | 291.9 | 21.5 |
| 200 | 10 | 310.2 | 310.0 | 325.1 | 342.3 | 32.2 |
| 200 | 20 | 425.5 | 425.2 | 458.2 | 481.9 | 47.0 |
| 300 | 1 | 187.1 | 185.5 | 197.8 | 220.5 | 5.3 |
| 300 | 5 | 347.9 | 347.7 | 381.0 | 410.1 | 14.4 |
| 300 | 10 | 459.9 | 456.4 | 487.6 | 520.9 | 21.7 |
| 300 | 20 | 657.3 | 641.9 | 744.5 | 1027.7 | 30.4 |
| 500 | 1 | 306.6 | 305.4 | 320.2 | 332.3 | 3.3 |
| 500 | 5 | 580.2 | 581.2 | 601.5 | 634.0 | 8.6 |
| 500 | 10 | 762.5 | 762.9 | 796.7 | 812.0 | 13.1 |
| 500 | 20 | 1091.6 | 1081.6 | 1223.3 | 1358.7 | 18.3 |

Concurrency slowdown (1→20 clients):
- 100 nonces: 3.43x (62ms → 213ms)
- 200 nonces: 3.50x (122ms → 426ms)
- 300 nonces: 3.51x (187ms → 657ms)
- 500 nonces: 3.56x (307ms → 1092ms)

### Decision

`MaxNonceRange` set to **200** based on these results:
- Sequential: 135ms avg at 200 nonces — well within request budget.
- Concurrent at 20 clients: 426ms avg, 458ms p95 — acceptable.
- 300 nonces at 20 concurrent hits ~750ms p95 — borderline.
- Growth is linear (per-nonce cost stable), so the limit is about total latency budget, not algorithmic scaling.

## Running

```bash
# Load tests (~12s)
go test -tags load -run TestLoad -v ./internal/attestation/...

# Stress tests (~70s)
go test -tags stress -run TestStress -v ./internal/attestation/teeavailabilitycheck/...

# PMWFeeProof benchmarks (requires Docker)
docker compose -f internal/tests/docker/docker-compose.yaml up -d --wait
go test -tags docker_bench -run TestBenchmarkFeeProofPostgres -v ./internal/attestation/pmwfeeproof/xrp/
go test -tags docker_bench -run TestBenchmarkFeeProofConcurrent -v ./internal/attestation/pmwfeeproof/xrp/
docker compose -f internal/tests/docker/docker-compose.yaml down
```
