<div align="center">
  <a href="https://flare.network/" target="blank">
    <img src="https://content.flare.network/Flare-2.svg" width="300" alt="Flare Logo" />
  </a>
  <br />
  Verifier service for Flare FDC2 attestation requests
  <br />
  <a href="#go-verifier-api">About</a>
  ·
  <a href="CONTRIBUTING.md">Contributing</a>
  ·
  <a href="SECURITY.md">Security</a>
</div>

# Go Verifier API


## Prerequisites to Run Verifier API
Each attestation type requires certain environment variables to be set. The following are common variables needed for all attestation types:
 ```env
PORT=<port_number>
API_KEYS=<comma_separated_strings>
```

> **NOTE**: The `<port_number>` value must be consistent with the `PORT` environment variable throughout the configuration.

### `TeeAvailabilityCheck` Attestation Type
Environment variables:
 ```env
VERIFIER_TYPE=TeeAvailabilityCheck
SOURCE_ID=TEE
RELAY_CONTRACT_ADDRESS=0x...
RPC_URL=https://<flare>

# Test/E2E-only flags (optional, default to false):
ALLOW_TEE_DEBUG=false
DISABLE_ATTESTATION_CHECK_E2E=false
ALLOW_PRIVATE_NETWORKS=false
```

> **NOTE**: `ALLOW_TEE_DEBUG`, `DISABLE_ATTESTATION_CHECK_E2E`, and `ALLOW_PRIVATE_NETWORKS` are test/E2E-only flags. In production, you should leave them unset (they default to false). `ALLOW_TEE_DEBUG=true` *additionally* accepts debug-mode TEEs alongside production TEEs (every debug admission logs a WARN); debug TEEs have the debugger attached and secrets are extractable, so this must never be set on production deployments. `ALLOW_PRIVATE_NETWORKS` permits private/loopback IPs (e.g. Docker bridge `172.17.0.1`) while still blocking dangerous IPs (link-local/metadata, multicast, Teredo, 6to4) and preserving DNS pinning.

> **WARNING: MagicPass bypass** — TEE nodes running in non-production mode (`settings.Mode != 0`) return `"magic_pass"` instead of a real attestation token. The verifier unconditionally accepts this token and skips ALL attestation validation (PKI, claims, CRL). This exists to support hackathon and development environments where real Google Confidential Space attestation is unavailable. **Do NOT rely on this in production** — any TEE returning this string will be trusted without verification.

The `TeeAvailabilityCheck` attestation type also uses Google Confidential Space Root Certificate, which is stored locally in the folder _internal/config/assets_. Read more about it [here](./internal/config/assets/README.md).

### `PMWMultisigAccountConfigured` Attestation Type
Environment variables:
```
VERIFIER_TYPE=PMWMultisigAccountConfigured
SOURCE_ID=testXRP
RPC_URL=https://<xrpl>
```

### `PMWPaymentStatus` Attestation Type
You will need to run following indexers:
- [xrp-indexer](https://github.com/flare-foundation/verifier-xrp-indexer)
- [c-chain indexer](https://github.com/flare-foundation/flare-system-c-chain-indexer) 

Environment variables:
```env
VERIFIER_TYPE=PMWPaymentStatus
SOURCE_ID=testXRP
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
SOURCE_DATABASE_URL=postgres://user:pass@host:port/db
FLARE_TEE_MANAGER_CONTRACT_ADDRESS=0x...
```

> **NOTE**: `FLARE_TEE_MANAGER_CONTRACT_ADDRESS` is the on-chain contract that emits `TeeInstructionsSent` events. The verifier rejects indexed logs emitted by any other address.

### `PMWFeeProof` Attestation Type
Requires the same indexers as `PMWPaymentStatus`.

Environment variables:
```env
VERIFIER_TYPE=PMWFeeProof
SOURCE_ID=testXRP
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
SOURCE_DATABASE_URL=postgres://user:pass@host:port/db
FLARE_TEE_MANAGER_CONTRACT_ADDRESS=0x...
```

## How to Set Up and Run Verifier
1. Fill in the `.env` file (for local development) or set environment variables directly (for production). To load the `.env` file at startup set `LOAD_DOTENV=true` in your shell before running the binary — `.env` loading is opt-in so production deployments are not sensitive to filesystem contents.

2. Install dependencies:

    ```bash
    go mod tidy
    ```

3. Run the project:
    ```bash
    go run ./cmd/main.go
    ```
    For local development with a `.env` file, set `LOAD_DOTENV=true` so the binary loads it at startup:
    ```bash
    LOAD_DOTENV=true go run ./cmd/main.go
    ```
    In production, leave `LOAD_DOTENV` unset and inject environment variables via the container runtime.

4. Access Swagger UI:
    ```
    http://localhost:<port_number>/api-doc
    ```
    Replace `<port_number>` with the value set in your `PORT` environment variable.

## API Reference
<b>Base path for all verifier endpoints</b>:
```
/verifier/<sourceName>/<attestationType>/
```
- `<sourceName>` must be lowercase.
- `<attestationType>` is the type of attestation (e.g., TeeAvailabilityCheck, PMWPaymentStatus, PMWMultisigAccountConfigured).

See [API reference](docs/api.md) for endpoint definitions and examples.

## Historical: TEE poller
An earlier version of `TeeAvailabilityCheck` ran a background poller that pinged active TEEs and maintained in-memory liveness samples. It was removed in favor of live-only verification. The last commit containing the full poller implementation is [`70d8c33`](../../commit/70d8c33a8c9cb886252e2e2413df7c530a4b05b6); check that commit out to inspect or restore the code.

## Attestation Request Submission
The process of submitting an attestation requests is as follows:

Attestation requests are triggered via TEE smart contracts. The TEE relay client, which acts as a connector between contracts on Flare's C-chain and TEE clients, listens to `TeeInstructionsSent` events with an `instructionId` that correspond to an attestation request (`FDC2_OP_TYPE` (`"F_FDC2"`) and `PROVE` (`"PROVE"`)). Each attestation request is then placed into a queue and gradually promoted to the designated verifier server. It is advised that each TEE relay client runs its own verifier server.

### Rate Limit
The blockchain itself limits how many attestation requests can be emitted per block, while the queue system enforces a controlled consumption rate for verifier servers. It is also expected that the person deploying the verifier server implements additional rate limiting at other levels.

### Security Headers
For internal-only APIs, we use a minimal set of headers:
- FrameDeny – prevent clickjacking
- ContentTypeNosniff – prevent MIME sniffing

Other headers (CORS, SSL redirect, STS, cross-origin policies) are not needed because these services are only accessed internally by trusted services, not browsers or public clients.

Minimal headers keep internal communication safe without unnecessary overhead.

## Running Tests
1. Running all tests with coverage
```bash
sh gencover.sh
```
The script is located in [gencover.sh](./gencover.sh).
- Docker services defined in [internal/tests/docker/docker-compose.yaml](./internal/tests/docker/docker-compose.yaml) will **automatically start**.
- All tests (unit + integration) will run.
- Docker services will **automatically shut down** after all tests complete.
This is the simplest way to run everything without worrying about Docker manually.

2. Running specific tests manually
- The majority of tests are **self-contained**:
    - Do **not require Docker** and can be run directly:
        ```bash
        go test -v <path_to_test>
        ```
- A few tests (PMWPaymentStatus / PMWFeeProof) access the indexer databases and are **Docker-dependent**. They are gated behind the `integration` build tag, so a bare `go test ./...` skips them and stays green without Docker.
    - Start Docker manually:
        ```bash
        docker compose -f internal/tests/docker/docker-compose.yaml up -d
        ```
    - Run the integration tests (note `-tags integration`):
        ```bash
        go test -tags integration -v <path_to_test>
        ```
    - Stop Docker after finishing:
        ```bash
        docker compose -f internal/tests/docker/docker-compose.yaml down
        ```

3. Running fuzz tests

    Fuzz tests run their seed corpus as regular tests during `go test` and `gencover.sh`. To run actual fuzzing with random inputs:
    ```bash
    go test ./internal/attestation/teeavailabilitycheck/verifier/ -fuzz FuzzResolveExternalURL -fuzztime 60s
    ```
    Available fuzz targets: `FuzzResolveExternalURL`, `FuzzGetOrFetchCRL`, `FuzzFetchCRLsForToken`, `FuzzFetchTEEChallengeResult`.

4. Running benchmarks

    Benchmark tests measure PMWFeeProof performance scaling with real Postgres + MySQL. They require Docker and are gated behind the `docker_bench` build tag:
    ```bash
    docker compose -f internal/tests/docker/docker-compose.yaml up -d
    # Sequential benchmark (single client, varying nonce ranges):
    go test -tags docker_bench -run TestBenchmarkFeeProofPostgres -v ./internal/attestation/pmwfeeproof/xrp/
    # Concurrent benchmark (multiple clients, varying nonce ranges):
    go test -tags docker_bench -run TestBenchmarkFeeProofConcurrent -v ./internal/attestation/pmwfeeproof/xrp/
    docker compose -f internal/tests/docker/docker-compose.yaml down
    ```

5. Running load tests

    Load tests are gated behind the `load` build tag and don't run during normal `go test` or `gencover.sh`:
    ```bash
    go test -tags load -run TestLoad -v ./internal/attestation/teeavailabilitycheck/verifier/ ./internal/attestation/pmwmultisigconfigured/xrp/ ./internal/attestation/pmwpaymentstatus/db/ ./internal/attestation/pmwpaymentstatus/xrp/ ./internal/attestation/pmwfeeproof/db/ ./internal/attestation/pmwfeeproof/xrp/
    ```

6. Running stress tests

    Stress tests are gated behind the `stress` build tag. They take longer (~70s) and push beyond normal load:
    ```bash
    go test -tags stress -run TestStress -v ./internal/attestation/teeavailabilitycheck/verifier/
    ```

    For detailed results, findings, and test parameters, see [docs/load-and-stress-tests.md](docs/load-and-stress-tests.md).

## TODO (to-think-about) list
- Other `TODO`s inside the code and README.
- TEEAvailabilityCheck currently supports only "google". When support for other platforms is added, TeeInfo.Platform needs to be added in order to know, how to decode the data.
- PMWFeeProof: Confirm with FAsset team that the `estimatedFee` formula (`pay_maxFee + sum(max(0, reissue_maxFee - pay_maxFee))`) is suitable for their fee reconciliation use case.
- `go.mod` pins `github.com/jackc/pgx/v5 v5.9.1` as an explicit indirect override because `gorm.io/driver/postgres v1.6.0` pulls the unpatched v5.6.0 (CVE-2026-33815, CVE-2026-33816). Drop the explicit pgx require once a newer `gorm.io/driver/postgres` ships that pulls pgx >= v5.9.0.

### Review findings to address (from code review, not yet fixed)

- **[Low/cosmetic] `FetchTEEChallengeResult` builds the request URL by string concatenation, assuming `baseURL` is a bare origin.**
  `internal/attestation/teeavailabilitycheck/verifier/verifier.go:353` does `url := fmt.Sprintf("%s/action/result/%s", baseURL, ...)`, while the dial target / Host header / TLS SNI are derived separately by parsing the same `baseURL` via `ResolveExternalURL` + `BuildPinnedAddr` (lines 354–358). This is **not** a security issue: the connection is always pinned to the IP that `ResolveExternalURL(baseURL)` validated, `Host`/SNI come from the resolved struct (not from `url`), and `url` is only used after validation passes (no TOCTOU). The fragility is purely in the path composition — a `baseURL` carrying a path prefix (`https://proxy.example.com/v1`) flows into the path (probably intended but undocumented), a trailing slash yields a double slash (`//action/result/...`), and a query/fragment in `baseURL` produces a malformed path. All require unusual operator config and at worst cause a failed request. Suggested fix: parse `baseURL` once with `url.Parse` and compose the path with `ResolveReference`/`path.Join` (also avoids parsing `baseURL` twice), **or** have `ResolveExternalURL` reject a non-empty `Path`/`RawQuery` and document that `baseURL` must be `scheme://host[:port]`. Pre-existing; not introduced by the new-signatures work.

- **[Low] `getBoolOrSetFalse` silently defaults invalid bool env values to `false` instead of failing the boot.**
  `getBoolOrSetFalse` (`internal/config/tee_availability_check.go:109`) parses the three boolean flags `ALLOW_TEE_DEBUG`, `DISABLE_ATTESTATION_CHECK_E2E`, `ALLOW_PRIVATE_NETWORKS` (used at lines 51–53). `strconv.ParseBool` only accepts `true/false/1/0/t/f/...`; any other non-empty value (e.g. `yes`, `on`, `1.0`, or a typo like `ture`) is swallowed, logged at WARN, and treated as `false`. This is **not** a security issue — every flag fails *closed* (a typo always lands on the strict/production side, never weakens it). The harm is operability: a typo silently discards operator intent with only a log line as signal. Concretely, `DISABLE_ATTESTATION_CHECK_E2E=ture` stays `false`, leaves attestation checks on, and then the required-vars check kills startup with `missing environment variables: TEE_AUDIENCE` — an error that points at the wrong cause (the real problem is the typo'd disable flag). Suggested fix: keep the empty-string branch defaulting to `false` (unset is valid), but make a **non-empty, unparseable** value return an error and propagate it out of `BuildTeeAvailabilityCheckConfig`, so misconfiguration fails loudly at boot with the offending key/value. Pre-existing; the new required-vars check (see the `TEE_AUDIENCE`/`TEE_ALLOWED_IMAGE_IDS` work) makes the misleading-failure case sharper.

## Technical Specification
See [docs/SPEC.md](docs/SPEC.md) for the full technical specification covering architecture, verification flows, error model, and configuration.
