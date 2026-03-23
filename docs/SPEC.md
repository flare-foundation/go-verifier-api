# Go Verifier API - Codebase Explanation and Technical Specification

## 1. Purpose
This service verifies attestation requests for Flare FDCv2 workflows and returns ABI-encoded responses.

It supports four attestation types:
- `TeeAvailabilityCheck`
- `PMWPaymentStatus`
- `PMWMultisigAccountConfigured`
- `PMWFeeProof`

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
- `PMWFeeProof`:
  - Constructs service + DB connections + verifier.
  - Registers endpoints.
  - Adds service (DB closer) to shutdown closers.

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
The `verify` and `prepareResponseBody` handlers classify verifier failures via `classifyVerifyError`:
- `422 Unprocessable Entity` for XRP RPC non-success status (e.g., account not found) — `ErrRPCNonSuccess`.
- `503 Service Unavailable` for XRP RPC network/transport failures — `ErrGetAccountInfo`.
- `500 Internal Server Error` for all other verifier errors.

## 6. Configuration Specification
## 6.1 Common required env vars
- `PORT`
- `API_KEYS` (comma-separated; trimmed; must contain at least one non-empty key)
- `VERIFIER_TYPE` (`TeeAvailabilityCheck`, `PMWPaymentStatus`, `PMWMultisigAccountConfigured`, `PMWFeeProof`)
- `SOURCE_ID` (`TEE`, `XRP`, `testXRP`)

## 6.2 Attestation-specific env vars
### TeeAvailabilityCheck
Required:
- `RPC_URL`
- `RELAY_CONTRACT_ADDRESS`
- `TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS`

Optional test/E2E flags:
- `ALLOW_TEE_DEBUG` (default false)
- `DISABLE_ATTESTATION_CHECK_E2E` (default false)
- `ALLOW_PRIVATE_NETWORKS` (default false) — test/E2E only. Allows private/loopback IPs while still blocking dangerous IPs and preserving DNS pinning. Useful for Docker bridge networking.

Also loads embedded Google root certificate:
- `internal/config/assets/google_confidential_space_root_20340116.crt`

### PMWPaymentStatus
Required:
- `SOURCE_DATABASE_URL` (Postgres)
- `CCHAIN_DATABASE_URL` (MySQL)

### PMWMultisigAccountConfigured
Required:
- `RPC_URL` (XRPL endpoint)

### PMWFeeProof
Required:
- `SOURCE_DATABASE_URL` (Postgres)
- `CCHAIN_DATABASE_URL` (MySQL)

## 7. Attestation Module Specs
## 7.1 TeeAvailabilityCheck
### Primary flow (`Verify`)
1. Validate and resolve proxy URL (SSRF protection + DNS rebinding prevention). When `ALLOW_PRIVATE_NETWORKS` is set, private/loopback IPs are allowed but dangerous IPs (link-local, metadata, multicast, Teredo, 6to4) are still blocked. DNS pinning is always active. Pin the resolved IP, then fetch challenge result from `{proxyURL}/action/result/{instructionID}` using the pinned connection.

### URL validation (`verifier/url_validation.go`)
Prevents SSRF by validating the TEE proxy URL before any request is made.

**Validation pipeline** (applied in order):
1. **Scheme**: only `http` and `https` allowed; no `file://`, `ftp://`, etc.
2. **Userinfo**: rejected (e.g. `http://user:pass@host`).
3. **Hostname**: `localhost` and `*.localhost` rejected (strict mode only).
4. **IP check**: if the host is an IP literal, it is checked directly. If it is a hostname, it is resolved via DNS (timeout: `750ms`) and **all** resolved IPs are checked.
5. **Pinning**: the first resolved IP is pinned — the HTTP connection dials the pinned IP directly via a custom `DialContext`, with the original hostname preserved in the HTTP `Host` header and TLS SNI `ServerName`. This prevents DNS rebinding (TOCTOU) where a second DNS lookup between validation and fetch could return a different IP.

**What each mode blocks:**

| | Strict (default) | `ALLOW_PRIVATE_NETWORKS=true` |
|---|---|---|
| `localhost` / `*.localhost` hostnames | Blocked | Allowed |
| Loopback (`127.0.0.0/8`, `::1`) | Blocked | Allowed |
| Private (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`) | Blocked | Allowed |
| Cloud metadata (`fd00:ec2::254`) | Blocked | Blocked |
| Link-local, multicast, unspecified (`0.0.0.0`, `::`) | Blocked | Blocked |
| Carrier-grade NAT (`100.64.0.0/10`) | Blocked | Blocked |
| Benchmark testing (`198.18.0.0/15`) | Blocked | Blocked |
| 6to4 (`2002::/16`), Teredo (`2001::/32`) | Blocked | Blocked |
| Documentation (`2001:db8::/32`), discard (`100::/64`) | Blocked | Blocked |
| DNS pinning | Active | Active |
2. Validate challenge equals request challenge.
3. Recover proxy signer and match `teeProxyId`.
4. **In parallel** (both depend only on the challenge response):
   - a. `DataVerification`: CRL fetch + PKI validation + TEE ID + claims (see below).
   - b. `CheckSigningPolicies`: validate signing policy hashes against relay contract (2 concurrent RPC calls).
5. Return status payload (`OK`/`OBSOLETE`/`DOWN`) with metadata.

### JWT attestation token validation (`DataVerification`)
The attestation token is a JWT signed by Google for Confidential Space TEEs.

**PKI validation:**
- Parsed and validated via `googlecloud.ParseAndValidatePKIToken()` using the embedded Google root certificate (`internal/config/assets/google_confidential_space_root_20340116.crt`).
- Verifies the full certificate chain back to Google's root.
- Intermediate and leaf certificates are checked against cached CRLs (see CRL revocation checking below).

**Claims validation (`ValidateClaims`):**
1. **EATNonce** — Exactly one nonce must be present and must equal the hex-encoded hash of the TeeInfo data.
2. **Debug status** — If `AllowTeeDebug=false` (production): requires `debugStatus == "disabled-since-boot"`. If `AllowTeeDebug=true` (testing): rejects production TEEs.
3. **Software name** — Must equal `"CONFIDENTIAL_SPACE"`.
4. **Stability** — If `SupportAttributes` is nil → hard error (verification fails). If present but `"STABLE"` not in the list → returns status `OBSOLETE`.
5. **CodeHash** — Extracted from `SubMods.Container.ImageDigest` (sha256 digest → 32-byte hash).
6. **Platform** — Extracted from `HWModel` claim (e.g. `"GCP_INTEL_TDX"` → 32-byte hash).

**Bypass (E2E):** Setting `DISABLE_ATTESTATION_CHECK_E2E=true` skips JWT validation entirely (E2E testing only).

**Bypass (MagicPass):** TEE nodes running in non-production mode (`settings.Mode != 0`) return `"magic_pass"` instead of a real attestation token. The verifier unconditionally accepts this token, skips all attestation validation (PKI, claims, CRL), and returns `OK` with hardcoded test values for `codeHash` and `platform`. This supports hackathon and development environments. Do not rely on this in production.

### Verify timeout budget
The [client](https://gitlab.com/flarenetwork/tee/tee-relay-client/-/blob/main/internal/router/processors/ftdc_verifier.go?ref_type=heads#L50) calls the verifier with a **10s timeout, 3 retries, 2s delay between retries**. The verifier targets a worst-case response time under 8s so the client can retry on transient failures.

| Phase | Timeout | Notes |
|---|---|---|
| URL validation (DNS) | 750ms | SSRF prevention, sequential |
| Challenge fetch | 4s | Main TEE proxy call incl. TLS handshake, sequential |
| CheckSigningPolicies (chain fetch) | 3s | RPC calls to Flare node, parallel with DataVerification |
| DataVerification | ~0ms | JWT parsing, no network call in prod, parallel with above |
| **Worst-case total** | **~7.75s** | |

Internal retry is set to 1 attempt (`chainMaxAttempts = 1`) — the client handles retries.

### CRL revocation checking
Intermediate and leaf certificates from the x5c chain are checked for revocation using CRLs.

**Responsibilities split:**

`go-flare-common` (validation logic — `pkg/tee/attestation/googlecloud/google_cloud.go`):
- `ParseAndValidatePKIToken(attestationToken, rootCert, leafCRL, intermediateCRL)` accepts pre-fetched CRLs as separate `*x509.RevocationList` parameters (nil when unavailable).
- `PKICertificates.Verify()` calls `verifyCRL()` after chain and lifetime checks.
- `checkCRL(name, cert, crl, issuer)` is called for each cert (leaf checked against intermediate as issuer, intermediate checked against root as issuer):
  1. If CRL is nil: log warning and skip (distinguishes "no CRL distribution points" vs "CRL not provided").
  2. Validate CRL time window (`ThisUpdate` ≤ now ≤ `NextUpdate`).
  3. Verify CRL signature against the issuer cert (`crl.CheckSignatureFrom(issuer)`).
  4. Reject if the cert's serial number appears in `RevokedCertificateEntries`.

`go-verifier-api` (fetching and caching — `verifier/crl_cache.go`):
- **Not a poller** — purely request-driven. `CRLCache.GetCRLsForToken()` is called inline during `DataVerification()` with the request `ctx`, before `ParseAndValidatePKIToken`.
- **Strict (all-or-nothing)**: if all CRL distribution points fail for either cert, verification fails.
- Parses the attestation token unverified (`ParsePKITokenUnverified`) to extract the x5c certificate chain. Before fetching CRLs, verifies the token's root certificate matches the trusted root (`GoogleRootCertificate`). Then reads `CRLDistributionPoints` from the leaf and intermediate certs.
- Leaf and intermediate CRL fetches run **in parallel**. For each cert, all distribution points are tried in order; the first successful fetch is used (fallback on fetch/parse/issuer-verification failure).
- **CRL issuer verification at fetch time**: after parsing, `crl.CheckSignatureFrom(issuer)` is called before caching. A CRL signed by a different CA is rejected and the next distribution point is tried.
- **Singleflight deduplication**: concurrent requests for the same CRL URL are deduplicated via `singleflight.Group` — only one HTTP fetch per URL, others wait for the result. This avoids redundant fetches when multiple data providers hit the verifier simultaneously.
- A **CRL cache** (keyed by URL, guarded by `sync.RWMutex`) avoids re-fetching on every verify call. Cached entries are considered fresh when all of: (1) less than `crlMaxCacheTTL` (4 hours) has elapsed since fetch, (2) the CRL's `NextUpdate` is not zero, and (3) `NextUpdate` has not passed. CRLs with a zero `NextUpdate` are always re-fetched. The TTL cap prevents stale cache when a CA publishes a new CRL (e.g. emergency revocation) before the old `NextUpdate`.
- On cache miss or stale entry, the CRL is fetched inline via `fetcher.GetBytes` (timeout: `2s`). The response is PEM-decoded if PEM-encoded (Google Cloud CRL endpoints return PEM), otherwise treated as raw DER, then parsed with `x509.ParseRevocationList`.
- Eviction: when the cache reaches `crlMaxEntries` (100), stale entries are purged; if still at capacity, the oldest entry is evicted to enforce the cap.
- The CRL cache is added to the shutdown closers for graceful cleanup (`CRLCache.Close()` clears the map).
- Note: Google CA Service only inserts the CRL Distribution Point (CDP) extension when CRL publication is enabled (`publish_crl` per-CA-pool setting); certs issued while CRL publication is disabled may have no CDP, so revocation checking must proceed without a CRL URL. Currently, the intermediate cert has a CDP but the leaf cert does not (no OCSP either). Google does not document or recommend CRL/OCSP checking for Confidential Space — their sample PKI token validation code only covers chain verification, root pinning, and signature checks.
  Sources:
  - CA Service CDP/publishing: `https://docs.cloud.google.com/certificate-authority-service/docs/managed-resources`
  - CA Service CA pool `publish_crl` setting: `https://docs.cloud.google.com/certificate-authority-service/docs/creating-ca-pool`
  - Confidential Space PKI validation (no CRL/OCSP in samples): `https://codelabs.developers.google.com/confidential-space-pki`
  - Confidential Space external resources (sample code): `https://docs.cloud.google.com/confidential-computing/confidential-space/docs/connect-external-resources`
  - Google OCSP deprecation (April 2025): `https://pki.goog/updates/april2025-ocsp-notice.html`

### Degraded flow when fetch fails
- Uses poller samples (`SamplesToConsider = 5`) for requested TEE.
- If all recent samples are invalid => returns `DOWN`.
- If insufficient samples => returns error.
- If any sample is valid or indeterminate => returns the original fetch error (TEE not confirmed DOWN).

### Poller behavior
- Runs on startup and every `SampleInterval = 1m`.
- Gets active TEEs from `TeeMachineRegistry` in chunks.
- Fetches each `/info` using a pinned connection, validates challenge freshness + claims + signing policies.
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

## 7.4 PMWFeeProof
Fee reconciliation attestation for PMW protocols. Compares estimated fees (from C-chain events) with actual fees (from XRP transactions) across a nonce range.

### Request
- `opType`, `senderAddress`, `fromNonce` (inclusive), `toNonce` (inclusive), `untilTimestamp` (Flare block timestamp cutoff for reissues).
- Nonce range capped at 100 (`MaxNonceRange`). Exceeding returns 400.

### Primary flow (`XRPVerifier.Verify`)
1. Validate nonce range.
2. Compute pay instruction IDs for all nonces, batch fetch C-chain events (`topic2 IN (?)`).
3. For each nonce: verify pay event exists, extract `maxFee`.
4. For each nonce: iteratively fetch reissue events (reissueNumber 0, 1, 2... until not found or `blockTimestamp > untilTimestamp`). Add residual `max(0, reissue_maxFee - pay_maxFee)`.
5. Sum as `estimatedFee`.
6. Batch fetch XRP transactions (`sequence IN (?)`), parse `Fee` field, sum as `actualFee`.
7. Return `{actualFee, estimatedFee}`.

### Error handling
- Missing pay event for any nonce → 422 (`ErrMissingPayEvent`).
- Missing XRP transaction for any nonce → 422 (`ErrMissingTransaction`).
- Nonce range too large → 400 (`ErrNonceRangeTooLarge`).
- DB infrastructure failure → 503 (via `ErrDatabase`).

### Architecture
- Standalone deployment, same pattern as PMWPaymentStatus.
- Reuses same data sources (Postgres source DB + MySQL C-chain index DB).
- Reuses `DecodeTeeInstructionsSentEventData` from `pmw_payment_status/instruction` (parameterized by `op.Command`).
- Reuses `GenerateInstructionID` from `pmw_payment_status/instruction` for pay IDs.
- See `docs/PMWFeeProof.md` for full spec including open questions.

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
  - nonce range too large or invalid — `ErrNonceRangeTooLarge` (PMWFeeProof)
- `401 Unauthorized`:
  - missing/invalid `X-API-KEY` (except `/api/health`)
- `422 Unprocessable Entity`:
  - XRP RPC returned non-success status (e.g., account not found) — `ErrRPCNonSuccess` (PMWMultisig)
  - requested record not found in database (instruction log or transaction) — `ErrRecordNotFound` (PMWPaymentStatus)
  - missing pay event for nonce — `ErrMissingPayEvent` (PMWFeeProof)
  - missing XRP transaction for nonce — `ErrMissingTransaction` (PMWFeeProof)
  - TEE data validation failed (challenge/proxy/claims/signing policy hash mismatch) — `ErrTEEDataValidation` (TEE)
  - RPC client-side errors (bad request, method not found) — `ErrInvalidInput` (TEE)
- `500 Internal Server Error`:
  - response encoding failures
  - URL validation errors (ambiguous — mix of bad URL and DNS issues) (TEE)
  - JSON decode errors in fetcher (TEE server returned invalid body) (TEE)
  - PMWPaymentStatus/PMWFeeProof data corruption (ABI decode, JSON unmarshal, malformed transaction data)
  - fallback for unexpected verifier errors (should not occur for PMWMultisig in practice)
- `503 Service Unavailable`:
  - XRP RPC network/transport failure (cannot reach XRPL node) — `ErrGetAccountInfo` (PMWMultisig)
  - database infrastructure failure (connection, timeout) — `ErrDatabase` (PMWPaymentStatus, PMWFeeProof)
  - insufficient poller samples to determine TEE status — `ErrInsufficientSamples` (TEE)
  - network errors from RPC calls — `ErrNetwork` (TEE)
  - RPC server-side errors — `ErrRPC` (TEE)
  - context deadline/canceled — `ErrContext` (TEE)
  - unclassified RPC errors (indeterminate → retry) — `ErrUnknown` (TEE)
  - HTTP request or non-OK status from TEE proxy — `ErrHTTPFetch` (TEE)
  - TEE action/result returned 404 (result not yet available in Redis) — `ErrActionResultNotFound` (TEE)

PMWMultisig verify errors are classified into `422` (`ErrRPCNonSuccess`) or `503` (`ErrGetAccountInfo`); the `500` default branch exists as a defensive fallback but is not reachable under normal operation. Note that PMWMultisig validation failures (wrong signers, wrong flags, etc.) do not return an HTTP error — they return a `200` response with `status=ERROR`. PMWPaymentStatus verify errors are classified into `422` (`ErrRecordNotFound`), `503` (`ErrDatabase`), or `500` (data corruption/unexpected errors). PMWFeeProof verify errors are classified into `400` (`ErrNonceRangeTooLarge`), `422` (`ErrMissingPayEvent`, `ErrMissingTransaction`), `503` (`ErrDatabase`), or `500` (data corruption/unexpected errors). TEE verify errors are classified into `422` (data validation), `503` (infrastructure/retry), or `500` (URL validation, JSON decode, unexpected errors).

## 10. Concurrency and State
- TEE `Verify` runs `DataVerification` and `CheckSigningPolicies` in parallel goroutines after the challenge fetch.
- `CheckSigningPolicies` fetches initial and last signing policy hashes in parallel goroutines.
- CRL leaf and intermediate fetches run in parallel goroutines within `GetCRLsForToken`.
- TEE poller uses worker pool (`defaultWorkerCount=10`) per cycle.
- Shared TEE sample cache guarded by RW mutex.
- Active TEE list cached and reused when chain query fails.
- CRL cache uses `sync.RWMutex` (RLock fast path for hits, WLock for inserts/eviction) and `singleflight.Group` to deduplicate concurrent fetches for the same URL.
- Config loaders use `sync.Once` singletons.

## 11. Testing Strategy in Repo
- Unit tests across API/config/attestation subpackages.
- Integration-style tests under `internal/tests/server`.
- Docker-based fixtures for payment-status dependencies (`internal/tests/docker/docker-compose.yaml`).
- `gencover.sh` orchestrates coverage + docker lifecycle.
- TEE availability server tests set `ALLOW_PRIVATE_NETWORKS=true` to allow `httptest` localhost URLs.

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
3. Close module resources (`DB`, poller, eth client, CRL cache).
