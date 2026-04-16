# Go Verifier API - Codebase Explanation and Technical Specification

## 1. Purpose
Verifies attestation requests for Flare FDC2 workflows; returns ABI-encoded responses. Supports four attestation types: `TeeAvailabilityCheck`, `PMWPaymentStatus`, `PMWMultisigAccountConfigured`, `PMWFeeProof`. At runtime the process serves exactly one attestation type + source pair.

## 2. System Context
- Language: Go (`module github.com/flare-foundation/go-verifier-api`)
- HTTP: `chi` router + `huma` OpenAPI handlers
- ABI/data: `go-ethereum` + `go-flare-common`
- Data stores (payment status, fee proof): PostgreSQL (source DB) + MySQL (C-chain index DB)
- RPC: Flare `ethclient` (TEE checks), XRPL RPC (multisig)

## 3. High-Level Architecture
### Entry and lifecycle
`cmd/main.go` loads env config and calls `api.RunServer`. `internal/api/server.go` builds router + Huma API, registers health and attestation routes via `LoadModule`, starts HTTP server, waits for `SIGINT/SIGTERM`, gracefully shuts down server and `io.Closer` dependencies.

### Module loading
`internal/api/loader.go` switches on `VERIFIER_TYPE`:

| Module | Constructs | Extra endpoint | Shutdown closers |
|---|---|---|---|
| `TeeAvailabilityCheck` | verifier + background poller | `GET /poller/tees` | poller, verifier |
| `PMWPaymentStatus` | service + 2 DB connections + verifier | — | payment service (DB closer) |
| `PMWMultisigAccountConfigured` | verifier | — | — |
| `PMWFeeProof` | service + 2 DB connections + verifier | — | service (DB closer) |

All modules register `verify` / `prepareRequestBody` / `prepareResponseBody`.

## 4. Routing and API Surface
### Global routes
- `GET /api/health` (no API key required)
- `GET /api-doc` and static swagger assets

### Attestation routes
Base: `/verifier/{sourceNameLower}/{attestationType}/`
- `POST .../prepareRequestBody`
- `POST .../prepareResponseBody`
- `POST .../verify`

TEE-only operational route: `GET /poller/tees`

### Request/response model
- Requests include encoded attestation/source IDs (`common.Hash`) and either `requestData` (for prepare request) or `requestBody` ABI bytes (for verify / prepare response).
- Responses return encoded `responseBody`; `prepareResponseBody` also returns decoded `responseData`.

## 5. Auth and Security Behavior
- **API key auth**: middleware checks `X-API-KEY` against `API_KEYS` env list; `/api/health` exempt; unauthorized → `401`.
- **Response security headers**: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` on all responses.
- **Request body size limit**: 1 MB (`maxRequestBodySize`); oversize rejected before processing.
- **Error sanitization**: `400`, `422`, `500`, `503` return only a generic message; full details logged server-side with a request ID for correlation.
- **Request ID correlation**: each handler request (prepareRequestBody, prepareResponseBody, verify) is assigned a unique ID, included in WARN/DEBUG server logs but never in HTTP response bodies. Unauthorized rejections log path + remote address.
- **Verify error classification** (`classifyVerifyError`): `422` for XRP RPC non-success (`ErrRPCNonSuccess`), `503` for XRP RPC network/transport (`ErrFetchAccountInfo`), `500` default for other verifier errors.

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
- `ALLOW_TEE_DEBUG` (default false) — when enabled, only accepts Google Confidential Space TEEs running in debug mode (`dbgstat != "disabled-since-boot"`) and rejects production TEEs. Intended for development/testing with debug TEE images.
- `DISABLE_ATTESTATION_CHECK_E2E` (default false) — when enabled, skips all JWT attestation validation (PKI, claims, CRL) in both the verify flow and the poller, returning hardcoded OK with test values. Intended for E2E tests without real Google attestation.
- `ALLOW_PRIVATE_NETWORKS` (default false) — test/E2E only. Allows private/loopback IPs while still blocking dangerous IPs and preserving DNS pinning. Useful for Docker bridge networking.
- `MAX_POLLED_TEES` (default 0) — controls how many TEEs the poller monitors. Extension 0 TEEs are always polled regardless of this cap. When 0 (default), only extension 0 is polled. When >0, the poller also includes TEEs from other extensions up to this limit.

Also loads embedded Google root certificate:
- `internal/config/assets/google_confidential_space_root_20340116.crt`

### PMWPaymentStatus
Required:
- `SOURCE_DATABASE_URL` (Postgres)
- `CCHAIN_DATABASE_URL` (MySQL)
- `TEE_INSTRUCTIONS_CONTRACT_ADDRESS` (canonical emitter of `TeeInstructionsSent`; instruction log queries include `AND address = ?`)

### PMWMultisigAccountConfigured
Required:
- `RPC_URL` (XRPL endpoint)

### PMWFeeProof
Required:
- `SOURCE_DATABASE_URL` (Postgres)
- `CCHAIN_DATABASE_URL` (MySQL)
- `TEE_INSTRUCTIONS_CONTRACT_ADDRESS` (canonical emitter of `TeeInstructionsSent`; instruction log queries include `AND address = ?`)

## 7. Attestation Module Specs

## 7.1 TeeAvailabilityCheck

### Primary flow (`Verify`)
1. Validate + resolve proxy URL (SSRF + DNS-rebinding prevention). With `ALLOW_PRIVATE_NETWORKS`, private/loopback IPs allowed but dangerous IPs (link-local, metadata, multicast, Teredo, 6to4) still blocked; DNS pinning always active. Pin resolved IP, fetch `{proxyURL}/action/result/{instructionID}` via pinned connection.
2. Validate challenge equals request challenge.
3. Recover proxy signer and match `teeProxyId`.
4. **In parallel** (both depend only on challenge response):
   - `DataVerification`: CRL fetch + PKI validation + TEE ID + claims.
   - `CheckSigningPolicies`: signing policy hashes against relay contract (2 concurrent RPC calls).
5. Return status (`OK`/`OBSOLETE`/`DOWN`) + metadata.

### URL validation (`verifier/url_validation.go`)
Pipeline: (1) scheme must be `http`/`https`; (2) userinfo rejected; (3) `localhost` / `*.localhost` rejected (strict mode only); (4) IP literal checked directly, hostname resolved via DNS (750ms timeout) with **all** resolved IPs checked; (5) first resolved IP pinned — HTTP connection dials pinned IP directly via custom `DialContext`, original hostname preserved in `Host` header and TLS SNI `ServerName` (prevents TOCTOU DNS rebinding).

| Category | Strict (default) | `ALLOW_PRIVATE_NETWORKS=true` |
|---|---|---|
| `localhost` / `*.localhost` hostnames | Blocked | Allowed |
| Loopback (`127.0.0.0/8`, `::1`) | Blocked | Allowed |
| Private (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`) | Blocked | Allowed |
| Cloud metadata, link-local, multicast, unspecified (`0.0.0.0`, `::`), "this network" (`0.0.0.0/8`), CGNAT (`100.64/10`), benchmark (`198.18/15`), NAT64 (`64:ff9b::/96`), 6to4 (`2002::/16`), Teredo (`2001::/32`), documentation (`2001:db8::/32`), discard (`100::/64`) | Blocked | Blocked |
| DNS pinning | Active | Active |

### JWT attestation token validation (`DataVerification`)
The attestation token is a JWT signed by Google for Confidential Space TEEs.

**PKI validation**: `googlecloud.ParseAndValidatePKIToken()` using the embedded Google root (`internal/config/assets/google_confidential_space_root_20340116.crt`). Verifies full chain back to root; intermediate + leaf checked against cached CRLs.

**Claims validation (`ValidateClaims`):**
1. **EATNonce** — Exactly one nonce must be present and must equal the hex-encoded hash of the TeeInfo data.
2. **Debug status** — If `AllowTeeDebug=false` (production): requires `debugStatus == "disabled-since-boot"`. If `AllowTeeDebug=true` (testing): rejects production TEEs.
3. **Software name** — Must equal `"CONFIDENTIAL_SPACE"`.
4. **Stability** — If `SupportAttributes` is nil → hard error (verification fails). If present but `"STABLE"` not in the list → returns status `OBSOLETE`.
5. **CodeHash** — Extracted from `SubMods.Container.ImageDigest` (sha256 digest → 32-byte hash).
6. **Platform** — Extracted from `HWModel` claim (e.g. `"GCP_INTEL_TDX"` → 32-byte hash).

**Bypasses**:
- `DISABLE_ATTESTATION_CHECK_E2E=true` — skips JWT validation entirely (E2E only).
- **MagicPass** — TEE nodes in non-production mode (`settings.Mode != 0`) return `"magic_pass"` instead of a real attestation token. The verifier unconditionally accepts it, skips all attestation validation (PKI, claims, CRL), and returns `OK` with hardcoded test values for `codeHash` and `platform`. Supports hackathon/dev environments; do not rely on in production.

### Verify timeout budget
The [client](https://github.com/flare-foundation/tee-relay-client/blob/main/internal/router/processors/fdc_verifier.go#L43) calls the verifier with a **10s timeout, 3 retries, 5s delay between retries** (20s total retry timeout). The verifier targets a worst-case response time under 8s so the client can retry on transient failures.

| Phase | Timeout | Notes |
|---|---|---|
| URL validation (DNS) | 750ms | SSRF prevention, sequential |
| Challenge fetch | 4s | Main TEE proxy call incl. TLS handshake, sequential |
| CheckSigningPolicies (chain fetch) | 3s | RPC calls to Flare node, parallel with DataVerification |
| DataVerification | ≤2s | CRL fetch on cache miss (leaf + intermediate in parallel, 2s each); ~0ms on warm cache. Parallel with above |
| **Worst-case total** | **~7.75s** | DataVerification is dominated by CheckSigningPolicies in the parallel window |

Internal retry is set to 1 attempt (`chainMaxAttempts = 1`) — the client handles retries.

### CRL revocation checking
Intermediate + leaf certs from the x5c chain are checked for revocation.

**Validation** (in `go-flare-common`, `pkg/tee/attestation/googlecloud/google_cloud.go`): `ParseAndValidatePKIToken(attestationToken, rootCert, leafCRL, intermediateCRL)` accepts pre-fetched CRLs (nil when unavailable). `PKICertificates.Verify()` calls `verifyCRL()` after chain/lifetime checks; per cert (leaf against intermediate, intermediate against root): if CRL nil → log + skip; else validate time window (`ThisUpdate` ≤ now ≤ `NextUpdate`), verify CRL signature (`CheckSignatureFrom(issuer)`), reject if serial in `RevokedCertificateEntries`.

**Fetching and caching** (`verifier/crl_cache.go`):
- Request-driven (not a poller). `CRLCache.GetCRLsForToken()` runs inline with request `ctx` before `ParseAndValidatePKIToken`.
- **Strict all-or-nothing**: if all CRL distribution points fail for either cert, verification fails.
- Parses token unverified (`ParsePKITokenUnverified`) to extract x5c. Before fetching, verifies the token's root matches the trusted root. Reads `CRLDistributionPoints` from leaf + intermediate.
- Leaf + intermediate fetches run **in parallel**. For each cert, distribution points tried in order; first successful fetch used. `CheckSignatureFrom(issuer)` is verified before caching — CRL signed by a different CA is rejected and the next DP is tried.
- **Singleflight** (`singleflight.Group`) deduplicates concurrent fetches for the same URL.
- **Cache** (`sync.RWMutex`, keyed by URL): an entry is fresh iff all of (a) age < `crlMaxCacheTTL` (4h), (b) `NextUpdate` non-zero, (c) `NextUpdate` not passed. Zero `NextUpdate` → always re-fetch. TTL cap guards against emergency revocation before the old `NextUpdate`.
- On miss/stale, fetched via `fetcher.FetchBytes` (2s timeout); PEM-decoded if PEM (Google Cloud CRL endpoints return PEM), else raw DER; parsed with `x509.ParseRevocationList`.
- Eviction: at `crlMaxEntries` (100), stale entries purged; if still full, oldest evicted.
- `CRLCache.Close()` added to shutdown closers.
- Google CA Service only inserts the CDP extension when `publish_crl` is enabled (per-CA-pool setting). Currently the intermediate cert has a CDP but the leaf does not (no OCSP either). Google does not document CRL/OCSP checking for Confidential Space — the sample PKI token validation code only covers chain verification, root pinning, and signature checks; revocation checking must tolerate missing CDPs. See Google CA Service and Confidential Space PKI documentation for details.

### Degraded flow when fetch fails
- Uses poller samples (`SamplesToConsider = 5`) for the requested TEE.
- Samples older than `MaxSampleStaleness` (`SamplesToConsider × SampleInterval`) → treated as insufficient → returns error.
- All recent samples invalid → returns `DOWN`.
- Insufficient samples → returns error.
- Any sample valid or indeterminate → returns the original fetch error (TEE not confirmed DOWN).

### Poller behavior
- Runs on startup and every `SampleInterval = 1m`.
- Fetches extension 0 TEEs via `getActiveTeeMachines(0)` (always polled). If `MAX_POLLED_TEES > 0`, also fetches remaining TEEs via `getAllActiveTeeMachines` and includes non-extension-0 TEEs up to the cap.
- Fetches each `/info` via pinned connection; validates challenge freshness + claims + signing policies.
- Rolling recent sample states in memory; exposed on `GET /poller/tees`.

### TEE status semantics
- Poller sample states: `VALID`, `INVALID`, `INDETERMINATE`.
- Verification response status values: `0 = OK`, `1 = OBSOLETE`, `2 = DOWN`.

## 7.2 PMWPaymentStatus

### Primary flow (`XRPVerifier.Verify`)
1. Build instruction ID from `(opType, PAY, sourceID, senderAddress, nonce)` using ABI packing + keccak.
2. Resolve `TeeInstructionsSent` event signature.
3. Fetch matching event log from C-chain index DB (`topic0`, `topic1=0`, `topic2=instructionID`).
4. Decode tee instruction message payload.
5. Query source DB transaction by `(source_address, sequence=nonce)`.
6. Parse raw source-chain transaction JSON. Reject if `TransactionType != "Payment"` — non-payment types (e.g. `AccountSet`, `TrustSet`) at the same `(sourceAddress, sequence)` cannot produce a payment status attestation.
7. Build FDC2 response:
   - recipient/token/amount/fee/reference from instruction message
   - status/revert reason from raw tx result
   - received amount for recipient — computed from `AffectedNodes` `AccountRoot` balance changes regardless of tx status (typically 0 for reverted txs, but computed from on-chain data rather than hardcoded). Native XRP only; issued-currency (IOU) payments that modify `RippleState` trust lines are not supported. Recipient address normalized from X-address to classic format before matching (XRPL metadata uses classic).
   - tx hash, fee, block number, timestamp from DB/tx data

### Data stores
- Source DB: transactions table (Postgres). C-chain DB: logs table (MySQL).

### Resource lifecycle
- Service owns 2 DB connections and closes both on shutdown.

## 7.3 PMWMultisigAccountConfigured

### Request validation
- `publicKeys` capped at 32 entries (XRPL `SignerList` protocol maximum); over → 400.
- Empty entries in `publicKeys` rejected → 400.

### Primary flow (`XRPVerifier.Verify`)
1. Call XRPL `account_info` with `ledger_index=validated`, `signer_lists=true`.
2. Resolve signer lists from response. XRPL API v1 (rippled) returns `signer_lists` inside `account_data`; API v2 and Clio return it at the `result` level — both layouts supported.
3. Validate signer list exists and matches provided pubkeys + threshold. Set-based comparison — duplicate `publicKeys` cannot mask extra on-chain signers.
4. Validate account flags: master key disabled; deposit auth disabled; destination tag requirement disabled; incoming XRP disallow disabled.
5. Validate no regular key set.
6. Success → `{status=OK, sequence}`; validation failure → `{status=ERROR, sequence=0}`.

### Public key handling
- Parsed and compressed secp256k1; converted to XRPL address for signer-set comparison.

## 7.4 PMWFeeProof
Fee reconciliation attestation for PMW protocols. Compares estimated fees (from C-chain events) with actual fees (from XRP transactions) across a nonce range.

### Request
- `opType`, `senderAddress`, `fromNonce` (inclusive), `toNonce` (inclusive), `untilTimestamp` (Flare block timestamp cutoff for reissues).
- Nonce range capped at 200 (`MaxNonceRange`); over → 400.

### Primary flow (`XRPVerifier.Verify`)
1. Validate nonce range.
2. Compute pay instruction IDs for all nonces; batch fetch C-chain events (`topic2 IN (?)`).
3. Per nonce: verify pay event exists, extract `maxFee`.
4. Per nonce: iteratively fetch reissue events (reissueNumber 0, 1, 2... until not found or `blockTimestamp > untilTimestamp`). Add residual `max(0, reissue_maxFee - pay_maxFee)`.
5. Sum as `estimatedFee`.
6. Batch fetch XRP transactions (`sequence IN (?)`), parse `Fee`, sum as `actualFee`.
7. Return `{actualFee, estimatedFee}`.

### Error handling
- Missing pay event for any nonce → 422 (`ErrMissingPayEvent`).
- Missing XRP transaction for any nonce → 422 (`ErrMissingTransaction`).
- Nonce range too large → 400 (`ErrNonceRangeTooLarge`).
- DB infrastructure failure → 503 (via `ErrDatabase`).

### Data retention
Both PMWPaymentStatus and PMWFeeProof depend entirely on indexer databases (no chain/RPC fallback). The XRP indexer retains transaction data for a configurable period (typically ~2 weeks in production); the C-chain indexer has its own retention policy. Requests outside retention → 422 for missing data. FDC2 attestation requests are tied to reward epochs with short deadlines, so out-of-retention requests indicate a protocol-level delay, not normal operation.

### Data stores
- Source DB: transactions table (Postgres). C-chain DB: logs table (MySQL).

## 8. ABI/Encoding Contract
- ABI schema source: connector contract metadata from `go-flare-common`.
- Each attestation type maps to request/response struct ABI names.
- `prepareRequestBody` converts JSON `requestData` → internal struct → ABI bytes.
- `verify` / `prepareResponseBody` decode request ABI bytes → internal structs.
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
  - XRP RPC network/transport failure (cannot reach XRPL node) — `ErrFetchAccountInfo` (PMWMultisig)
  - database infrastructure failure (connection, timeout) — `ErrDatabase` (PMWPaymentStatus, PMWFeeProof)
  - insufficient poller samples to determine TEE status — `ErrInsufficientSamples` (TEE)
  - network errors from RPC calls — `ErrNetwork` (TEE)
  - RPC server-side errors — `ErrRPC` (TEE)
  - context deadline/canceled — `ErrContext` (TEE)
  - unclassified RPC errors (indeterminate → retry) — `ErrUnknown` (TEE)
  - HTTP request or non-OK status from TEE proxy — `ErrHTTPFetch` (TEE)
  - TEE action/result returned 404 (result not yet available in Redis) — `ErrActionResultNotFound` (TEE)

Notes: PMWMultisig's `500` default branch is defensive and not reachable under normal operation. PMWMultisig validation failures (wrong signers, wrong flags, etc.) return a `200` with `status=ERROR`, not an HTTP error.

## 10. Concurrency and State
- **Parallelism**: TEE `Verify` runs `DataVerification` + `CheckSigningPolicies` concurrently after the challenge fetch; `CheckSigningPolicies` fetches initial + last signing policy hashes concurrently; CRL leaf + intermediate fetches run concurrently inside `GetCRLsForToken`.
- **TEE poller**: worker pool (`defaultWorkerCount=10`) per cycle.
- **Caches**: TEE sample cache guarded by RW mutex; active TEE list cached + reused when chain query fails; CRL cache uses `sync.RWMutex` (RLock fast path for hits, WLock for inserts/eviction) + `singleflight.Group` to dedupe concurrent fetches for the same URL.
- **Config loaders**: `sync.Once` singletons.

## 11. Testing Strategy in Repo
- Unit tests across API/config/attestation subpackages.
- Integration-style tests under `internal/tests/server`.
- Docker-based fixtures for payment-status deps (`internal/tests/docker/docker-compose.yaml`).
- `gencover.sh` orchestrates coverage + docker lifecycle.
- TEE availability server tests set `ALLOW_PRIVATE_NETWORKS=true` to allow `httptest` localhost URLs.

## 12. Operational Notes and Risks
- Poller sample cache is in-memory only by design choice (lost on restart).
- `PMWPaymentStatus` request includes `subNonce`, but current DB query path primarily keys by source address + nonce. XRP does not use batch payments, so each nonce maps to exactly one transaction. SubNonce filtering will be needed when UTXO chains are supported.

### Accepted risks
- **MagicPass bypass** (`verifier.go`): TEE nodes in non-production mode return `"magic_pass"` instead of a real attestation token. The verifier unconditionally accepts it and skips all attestation validation. Gated by the TEE node's `settings.Mode` — the verifier itself has no toggle. Compensating control: production TEE nodes never set `Mode != 0`. See §7.1.
- **Unauthenticated Swagger UI** (`/api-doc`): The OpenAPI documentation endpoint is intentionally exempt from API key auth to allow internal developers and auditors to browse the API. Compensating control: service is deployed behind internal infrastructure, not exposed to the public internet. No sensitive data is served on this endpoint.
- **HTTP redirects disabled** (`fetcher.go`): HTTP clients reject all redirects (`CheckRedirect` returns `ErrRedirect`). TEE proxy URLs are expected to resolve directly — TEE nodes cannot follow redirects on their POST-based proxy communication, so operators already configure non-redirecting URLs. Eliminates the SSRF bypass vector where a redirect target could point to a private/metadata IP.
- **Unbounded ABI event data decoding** (`instruction_event.go`): `DecodeTeeInstructionsSentEventData` decodes `log.Data` without an explicit size cap. Bounded in practice by: (1) C-chain block gas limits constrain the maximum emitted event size; (2) the C-chain indexer only stores logs from configured contract addresses; (3) emitting a large event costs significant gas. A future hardening step could add an explicit `len(log.Data)` check before ABI decoding.

## 13. Minimal Runtime Sequences
**Start**: load env → validate common config → build module-specific config → build verifier/service dependencies → register endpoints + auth middleware → start HTTP server → (TEE only) start background poller.

**Shutdown**: receive OS signal → HTTP graceful shutdown (`10s`) → close module resources (DB, poller, eth client, CRL cache).
