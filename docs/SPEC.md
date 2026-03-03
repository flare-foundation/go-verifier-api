# Go Verifier API - Codebase Explanation and Technical Specification

## 1. Purpose
This service verifies attestation requests for Flare FDCv2 workflows and returns ABI-encoded responses.

It supports three attestation types:
- `TeeAvailabilityCheck`
- `PMWPaymentStatus`
- `PMWMultisigAccountConfigured`

At runtime, the process is configured to serve exactly one attestation type + source pair.

## 2. System Context
- Language: Go (`module github.com/flare-foundation/go-verifier-api`)
- HTTP stack: `chi` router + `huma` OpenAPI handlers
- ABI/data primitives: `go-ethereum` + `go-flare-common`
- Data stores (payment status): PostgreSQL (source DB) + MySQL (C-chain index DB)
- Blockchain/RPC dependencies:
  - Flare RPC (`ethclient`) for TEE checks
  - XRPL RPC for multisig checks

## 3. High-Level Architecture
### Entry and lifecycle
- `cmd/main.go` loads env config and calls `api.RunServer`.
- `internal/api/server.go`:
  - Builds router and Huma API.
  - Registers health route and attestation routes via `LoadModule`.
  - Starts HTTP server.
  - Waits for `SIGINT/SIGTERM`.
  - Gracefully shuts down server and `io.Closer` dependencies.

### Module loading
`internal/api/loader.go` chooses module by `VERIFIER_TYPE`:
- `TeeAvailabilityCheck`:
  - Constructs verifier.
  - Registers verify/prepare endpoints.
  - Starts background TEE poller.
  - Registers `/poller/tees` endpoint.
  - Adds poller + verifier to shutdown closers.
- `PMWPaymentStatus`:
  - Constructs service + DB connections + verifier.
  - Registers endpoints.
  - Adds payment service (DB closer) to shutdown closers.
- `PMWMultisigAccountConfigured`:
  - Constructs verifier.
  - Registers endpoints.

## 4. Routing and API Surface
### Global routes
- `GET /api/health` (no API key required)
- `GET /api-doc` and static swagger assets

### Attestation routes
Base:
- `/verifier/{sourceNameLower}/{attestationType}/`

Per module endpoints:
- `POST .../prepareRequestBody`
- `POST .../prepareResponseBody`
- `POST .../verify`

TEE-only operational route:
- `GET /poller/tees`

### Request/response model
- Requests include encoded attestation/source IDs (`common.Hash`) and either:
  - `requestData` (for prepare request), or
  - `requestBody` ABI bytes (for verify/prepare response).
- Responses return encoded `responseBody`, and `prepareResponseBody` also returns decoded `responseData`.

## 5. Auth and Security Behavior
### API key auth
- Middleware checks `X-API-KEY` against `API_KEYS` env list.
- Exempt path: `/api/health`.
- Unauthorized responses: HTTP `401`.

### Response security headers
All responses get:
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`

### Important note
The `verify` and `prepareResponseBody` handlers map verifier failures to HTTP `500` via `warnHuma500`.

## 6. Configuration Specification
## 6.1 Common required env vars
- `PORT`
- `API_KEYS` (comma-separated; trimmed; must contain at least one non-empty key)
- `VERIFIER_TYPE` (`TeeAvailabilityCheck`, `PMWPaymentStatus`, `PMWMultisigAccountConfigured`)
- `SOURCE_ID` (`TEE`, `XRP`, `testXRP`)

## 6.2 Attestation-specific env vars
### TeeAvailabilityCheck
Required:
- `RPC_URL`
- `RELAY_CONTRACT_ADDRESS`
- `TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS`

Optional test/e2e flags:
- `ALLOW_TEE_DEBUG` (default false)
- `DISABLE_ATTESTATION_CHECK_E2E` (default false)
- `DISABLE_URL_VALIDATION` (default false)

Also loads embedded Google root certificate:
- `internal/config/assets/google_confidential_space_root_20340116.crt`

### PMWPaymentStatus
Required:
- `SOURCE_DATABASE_URL` (Postgres)
- `CCHAIN_DATABASE_URL` (MySQL)

### PMWMultisigAccountConfigured
Required:
- `RPC_URL` (XRPL endpoint)

## 7. Attestation Module Specs
## 7.1 TeeAvailabilityCheck
### Primary flow (`Verify`)
1. Validate and resolve proxy URL (SSRF protection + DNS rebinding prevention, unless `DISABLE_URL_VALIDATION` is set), pin the resolved IP, then fetch challenge result from `{proxyURL}/action/result/{instructionID}` using the pinned connection.

### URL validation (`verifier/url_validation.go`)
Prevents SSRF by validating the TEE proxy URL before any request is made:
- **Scheme**: only `http` and `https` allowed; no `file://`, `ftp://`, etc.
- **Userinfo**: rejected (e.g. `http://user:pass@host`).
- **Hostname**: `localhost` and `*.localhost` blocked.
- **IP resolution and pinning**: hostname is resolved via DNS (timeout: `750ms`) and **all** resolved IPs are checked — rejected if any resolves to a private/local IP. The first resolved IP is then pinned: the HTTP connection dials the pinned IP directly via a custom `DialContext`, with the original hostname preserved in the HTTP `Host` header and TLS SNI `ServerName`. This prevents DNS rebinding (TOCTOU) where a second DNS lookup between validation and fetch could return a different IP.
- **Blocked IPs** (via `net/netip` stdlib): loopback, private (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`), link-local unicast/multicast, multicast, unspecified (`0.0.0.0`, `::`).
- **Additional blocked prefixes**: carrier-grade NAT (`100.64.0.0/10`), benchmark testing (`198.18.0.0/15`), documentation (`2001:db8::/32`), discard (`100::/64`), 6to4 (`2002::/16` — can embed private IPv4), Teredo (`2001::/32` — can tunnel private IPv4).
2. Validate challenge equals request challenge.
3. Recover proxy signer and match `teeProxyId`.
4. **In parallel** (both depend only on the challenge response):
   - a. `DataVerification`: PKI validation + TEE ID + claims.
   - b. `CheckSigningPolicies`: validate signing policy hashes against relay contract (2 concurrent RPC calls).
5. Return status payload (`OK`/`OBSOLETE`/`DOWN`) with metadata.

### JWT attestation token validation (`DataVerification`)
The attestation token is a JWT signed by Google for Confidential Space TEEs.

**PKI validation:**
- Parsed and validated via `googlecloud.ParseAndValidatePKIToken()` using the embedded Google root certificate (`internal/config/assets/google_confidential_space_root_20340116.crt`).
- Verifies the full certificate chain back to Google's root.

**Claims validation (`ValidateClaims`):**
1. **EATNonce** — Exactly one nonce must be present and must equal the hex-encoded hash of the TeeInfo data.
2. **Debug status** — If `AllowTeeDebug=false` (production): requires `debugStatus == "disabled-since-boot"`. If `AllowTeeDebug=true` (testing): rejects production TEEs.
3. **Software name** — Must equal `"CONFIDENTIAL_SPACE"`.
4. **Stability** — If `SupportAttributes` is nil → hard error (verification fails). If present but `"STABLE"` not in the list → returns status `OBSOLETE`.
5. **CodeHash** — Extracted from `SubMods.Container.ImageDigest` (sha256 digest → 32-byte hash).
6. **Platform** — Extracted from `HWModel` claim (e.g. `"GCP_INTEL_TDX"` → 32-byte hash).

**Bypass:** Setting `DISABLE_ATTESTATION_CHECK_E2E=true` skips JWT validation entirely (E2E testing only).

### Degraded flow when fetch fails
- Uses poller samples (`SamplesToConsider = 5`) for requested TEE.
- If all recent samples are invalid => returns `DOWN`.
- If insufficient samples => returns error.
- If any sample is valid or indeterminate => returns the original fetch error (TEE not confirmed DOWN).

### Poller behavior
- Runs on startup and every `SampleInterval = 1m`.
- Gets active TEEs from `TeeMachineRegistry` in chunks.
- Fetches each `/info` (using pinned connection when URL validation is enabled), validates challenge freshness + claims + signing policies.
- Stores rolling recent sample states in memory.
- Exposes samples on `GET /poller/tees`.

### TEE status semantics
- Poller sample states: `VALID`, `INVALID`, `INDETERMINATE`.
- Verification response status values:
  - `0 = OK`
  - `1 = OBSOLETE`
  - `2 = DOWN`

## 7.2 PMWPaymentStatus
### Primary flow (`XRPVerifier.Verify`)
1. Build instruction ID from `(opType, PAY, sourceID, senderAddress, nonce)` using ABI packing + keccak.
2. Resolve `TeeInstructionsSent` event signature.
3. Fetch matching event log from C-chain index DB (`topic0`, `topic1=0`, `topic2=instructionID`).
4. Decode tee instruction message payload.
5. Query source DB transaction by `(source_address, sequence=nonce)`.
6. Parse raw source-chain transaction JSON.
7. Build FDCv2 response:
   - recipient/token/amount/fee/reference from instruction message
   - status/revert reason from raw tx result
   - received amount for recipient
   - tx hash, fee, block number, timestamp from DB/tx data

### Data stores
- Source DB: transactions table (Postgres)
- C-chain DB: logs table (MySQL)

### Resource lifecycle
- Service owns 2 DB connections and closes both on shutdown.

## 7.3 PMWMultisigAccountConfigured
### Primary flow (`XRPVerifier.Verify`)
1. Call XRPL `account_info` with `ledger_index=validated`, `signer_lists=true`.
2. Validate account signer list exists and matches provided pubkeys + threshold.
3. Validate account flags:
   - master key disabled
   - deposit auth disabled
   - destination tag requirement disabled
   - incoming XRP disallow disabled
4. Validate no regular key set.
5. Return `{status=OK, sequence}` on success.
6. Return `{status=ERROR, sequence=0}` on validation failure.

### Public key handling
- Public keys are parsed and compressed secp256k1.
- Converted to XRPL address for signer-set comparison.

## 8. ABI/Encoding Contract
- ABI schema source: connector contract metadata from `go-flare-common`.
- Each attestation type maps to request/response struct ABI names.
- `prepareRequestBody` converts JSON `requestData` -> internal struct -> ABI bytes.
- `verify` / `prepareResponseBody` decode request ABI bytes to internal structs.
- Handlers enforce request attestation/source IDs equal server-configured encoded IDs.

## 9. Error Model (Implementation)
- `400 Bad Request`:
  - attestation/source mismatch
  - invalid request body
  - decode/encode request conversion issues
- `401 Unauthorized`:
  - missing/invalid `X-API-KEY` (except `/api/health`)
- `500 Internal Server Error`:
  - verifier failures
  - response encoding failures

TEE-specific fetch/RPC/network errors are internally classified for poller sample state but generally surface as `500` in HTTP verify handlers.

## 10. Concurrency and State
- TEE `Verify` runs `DataVerification` and `CheckSigningPolicies` in parallel goroutines after the challenge fetch.
- `CheckSigningPolicies` fetches initial and last signing policy hashes in parallel goroutines.
- TEE poller uses worker pool (`defaultWorkerCount=10`) per cycle.
- Shared TEE sample cache guarded by RW mutex.
- Active TEE list cached and reused when chain query fails.
- Config loaders use `sync.Once` singletons.

## 11. Testing Strategy in Repo
- Unit tests across API/config/attestation subpackages.
- Integration-style tests under `internal/tests/server`.
- Docker-based fixtures for payment-status dependencies (`internal/tests/docker/docker-compose.yaml`).
- `gencover.sh` orchestrates coverage + docker lifecycle.
- TEE availability server tests set `DISABLE_URL_VALIDATION=true` to allow `httptest` localhost URLs.

## 12. Operational Notes and Risks
- `go.mod` includes local replace:
  - `github.com/flare-foundation/tee-node => ../tee-node`
  - build requires sibling `../tee-node` checkout.
- Poller sample cache is in-memory only by design choice (lost on restart).
- `PMWPaymentStatus` request includes `subNonce`, but current DB query path primarily keys by source address + nonce.

## 13. Minimal Runtime Sequences
### Start sequence
1. Load env.
2. Validate common config.
3. Build module-specific config.
4. Build verifier/service dependencies.
5. Register endpoints + auth middleware.
6. Start HTTP server.
7. (TEE only) start background poller.

### Shutdown sequence
1. Receive OS signal.
2. HTTP graceful shutdown (`10s`).
3. Close module resources (`DB`, poller, eth client).
