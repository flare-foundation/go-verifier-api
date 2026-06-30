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

| Module | Constructs | Shutdown closers |
|---|---|---|
| `TeeAvailabilityCheck` | verifier | verifier |
| `PMWPaymentStatus` | service + 2 DB connections + verifier | payment service (DB closer) |
| `PMWMultisigAccountConfigured` | verifier | — |
| `PMWFeeProof` | service + 2 DB connections + verifier | service (DB closer) |

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

**`VERIFIER_TYPE` × `SOURCE_ID`:** `VERIFIER_TYPE` and `SOURCE_ID` are first validated independently against the allowlists above, then each module preflights its `SOURCE_ID` at config/service construction and fails the boot on an unsupported value: TeeAvailabilityCheck accepts only `TEE`; the PMW modules accept only `XRP`/`testXRP`. A mismatched pair therefore fails fast at startup with a clear error rather than booting clean and rejecting every request with a 400 source-id mismatch. (Valid pairings: `TeeAvailabilityCheck`↔`TEE`; `PMWPaymentStatus`/`PMWMultisigAccountConfigured`/`PMWFeeProof`↔`XRP`/`testXRP`.)

## 6.2 Attestation-specific env vars
### TeeAvailabilityCheck
Required:
- `RPC_URL`
- `RELAY_CONTRACT_ADDRESS`
- `CHAIN_ID` — EVM chain ID this verifier serves; the attested `TeeInfo.ChainID` must equal it. Required and must be non-zero (the chain pin is enforced unconditionally; see §7.1).

Optional:
- `TEE_AUDIENCE` — override for the expected `aud` claim on Confidential Space attestation tokens. Defaults to `config.DefaultTeeAudience` (`"https://sts.google.com"`, the audience tee-node requests) when unset; set it only if tee-node's requested audience diverges.

Optional test/E2E flags:
- `ALLOW_TEE_DEBUG` (default false) — permissive flag. `false`: only production Confidential Space TEEs (`dbgstat == "disabled-since-boot"`) are accepted. `true`: production AND debug TEEs (`dbgstat != "disabled-since-boot"`) are both accepted; every debug admission emits a WARN log. Debug TEEs have the debugger attached and secrets can be extracted — never enable on production deployments. Intended for staging/E2E only.
- `DISABLE_ATTESTATION_CHECK_E2E` (default false) — when enabled, skips all JWT attestation validation (PKI, claims, CRL) in the verify flow, returning hardcoded OK with test values. Intended for E2E tests without real Google attestation.
- `ALLOW_PRIVATE_NETWORKS` (default false) — test/E2E only. Allows private/loopback IPs while still blocking dangerous IPs and preserving DNS pinning. Useful for Docker bridge networking.

Also loads embedded Google root certificate:
- `internal/config/assets/google_confidential_space_root_20340116.crt`

### PMWPaymentStatus
Required:
- `SOURCE_DATABASE_URL` (Postgres)
- `CCHAIN_DATABASE_URL` (MySQL)
- `FLARE_TEE_MANAGER_CONTRACT_ADDRESS` (canonical emitter of `TeeInstructionsSent`; instruction log queries include `AND address = ?`)
- `TEE_PAYMENTS_CONTRACT_ADDRESS` (the source's per-source `TeePayments` contract; `getInitialNonce` is called on it for the sequence binding — distinct from the FlareTeeManager diamond above)
- `RPC_URL` (Flare C-chain EVM RPC; read-only `TeePayments.getInitialNonce` for the deterministic paymentId→sequence binding)

### PMWMultisigAccountConfigured
Required:
- `RPC_URL` (XRPL endpoint)

### PMWFeeProof
Required:
- `SOURCE_DATABASE_URL` (Postgres)
- `CCHAIN_DATABASE_URL` (MySQL)
- `FLARE_TEE_MANAGER_CONTRACT_ADDRESS` (canonical emitter of `TeeInstructionsSent`; instruction log queries include `AND address = ?`)
- `TEE_PAYMENTS_CONTRACT_ADDRESS` (the source's per-source `TeePayments` contract; `getInitialNonce` is called on it for the sequence binding — distinct from the FlareTeeManager diamond above)
- `RPC_URL` (Flare C-chain EVM RPC; read-only `TeePayments.getInitialNonce` for the deterministic paymentId→sequence binding)

## 7. Attestation Module Specs

## 7.1 TeeAvailabilityCheck

### Primary flow (`Verify`)
1. Validate + resolve proxy URL (SSRF + DNS-rebinding prevention). With `ALLOW_PRIVATE_NETWORKS`, private/loopback IPs allowed but dangerous IPs (link-local, metadata, multicast, Teredo, 6to4, IPv4-compatible IPv6) still blocked; DNS pinning always active. Pin resolved IP, fetch `{proxyURL}/action/result/{instructionID}` via pinned connection.
2. Validate challenge equals request challenge.
   - **Chain pin**: require `response.TeeInfo.ChainID == CHAIN_ID`. The signatures are reconstructed using the attested `ChainID`, so this is the explicit check that it is the chain we serve (not merely internally consistent), closing cross-chain replay beyond the per-request challenge binding. Enforced **unconditionally** — the `DISABLE_ATTESTATION_CHECK_E2E` and `magic_pass` bypasses disable Google attestation validation, not chain identity. `CHAIN_ID` is required and non-zero for every deployment (0 is not a valid EVM chain ID, so there is no default).
3. Verify action-result integrity:
   - Recover proxy signer from `actionResp.ProxySignature` over `csigning.NewPayload(ProxyActionResult, chainID, Result.Hash()).Hash()`; require it to equal `req.teeProxyId`. `chainID` is `response.TeeInfo.ChainID` from the attestation payload.
   - Require `actionResp.Result.ID == req.instructionId`.
   - Require `actionResp.Result.Status == 1` (success for both availability-response producers in tee-node).
   - Require `(actionResp.Result.OPType, actionResp.Result.OPCommand) == (op.Reg, op.TEEAttestation)` — emitted by `VerificationFacet.requestTeeAttestation` and `MachineManagerFacet` during admission, handled by tee-node's `regutils.TEEAttestation` (instruction processor, `immediateResult=true`). The FDC2 availability-check flow (`VerificationFacet.requestAvailabilityCheckAttestation` → `Verification.requestFdc2Attestation` → relay → verifier) embeds the prior admission `instructionId` in its request body, so the verifier always fetches the admission action result; `(op.Get, op.TEEInfo)` is the only other tee-node pair that produces a `TeeInfoResponse`, but it is reachable only via the proxy's API-key-gated `/direct` endpoint and is not part of the trusted attestation flow, so it is excluded from the allowlist.
   - Verify `actionResp.Signature` over `csigning.NewPayload(TEEActionResult, chainID, Result.Hash()).Hash()` against `req.teeId` (TEE proof-of-possession on the action result).

   Both signatures use domain-separated, chain-bound preimages from `go-flare-common/pkg/signing`:
   - Domain tags (`TEEActionResult`, `ProxyActionResult`) prevent cross-purpose replay — a TEE signature can't be passed off as a proxy signature over the same bytes, or vice versa.
   - `chainID` binding prevents cross-chain replay — an action result signed on chain A can't be replayed on chain B. The chainID is read from `response.TeeInfo.ChainID` (carried inside the attestation payload, not trusted from the request). A wrong chainID simply fails signer recovery (fail-closed), since only the genuine TEE/proxy can sign over its real chain ID.

   `Result.Hash()` binds `Data`, `ID`, `SubmissionTag`, and `Status`, but **not** `OPType` or `OPCommand` — those are checked explicitly against `expectedAvailabilityOPType`/`expectedAvailabilityOPCommand`. `SubmissionTag` is signature-bound so it cannot be tampered with by the proxy; no separate verifier-side check is enforced because the submission convention is set by the proxy / relay client.
4. **In parallel** (both depend only on challenge response):
   - `DataVerification`: CRL fetch + PKI validation + TEE ID + claims.
   - `CheckSigningPolicies`: signing policy hashes against relay contract (2 concurrent RPC calls).
5. Return status (`OK`/`OBSOLETE`) + metadata. When the live fetch fails, the verifier returns the wrapped error; the caller / relay handles retry.

### URL validation (`verifier/url_validation.go`)
Pipeline: (1) scheme must be `http`/`https`; (2) userinfo rejected; (3) `localhost` / `*.localhost` rejected (strict mode only); (4) IP literal checked directly, hostname resolved via DNS (750ms timeout) with **all** resolved IPs checked; (5) first resolved IP pinned — HTTP connection dials pinned IP directly via custom `DialContext`, original hostname preserved in `Host` header and TLS SNI `ServerName` (prevents TOCTOU DNS rebinding).

| Category | Strict (default) | `ALLOW_PRIVATE_NETWORKS=true` |
|---|---|---|
| `localhost` / `*.localhost` hostnames | Blocked | Allowed |
| Loopback (`127.0.0.0/8`, `::1`) | Blocked | Allowed |
| Private (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`) | Blocked | Allowed |
| Cloud metadata, link-local, multicast, unspecified (`0.0.0.0`, `::`), "this network" (`0.0.0.0/8`), CGNAT (`100.64/10`), benchmark (`198.18/15`), NAT64 (`64:ff9b::/96`), 6to4 (`2002::/16`), Teredo (`2001::/32`), IPv4-compatible IPv6 (`::/96`, excl. `::1`), documentation (`2001:db8::/32`), discard (`100::/64`) | Blocked | Blocked |
| DNS pinning | Active | Active |

### JWT attestation token validation (`DataVerification`)
The attestation token is a JWT signed by Google for Confidential Space TEEs.

**PKI validation**: `googlecloud.ParseAndValidatePKIToken(token, rootCert, leafCRL, intermediateCRL, policy)` using the embedded Google root (`internal/config/assets/google_confidential_space_root_20340116.crt`). Verifies full chain back to root; intermediate + leaf checked against cached CRLs. The 5th argument is a `googlecloud.Policy` that drives JWT-level acceptance.

**`googlecloud.Policy` construction** (in `DataVerification`):

| Field | Value | Source |
|---|---|---|
| `Issuer` | `googlecloud.ConfidentialSpaceIssuer` | constant |
| `Audience` | `v.Cfg.TeeAudience` | `TEE_AUDIENCE` env var, or `config.DefaultTeeAudience` when unset |
| `EATNonce` | `hex.EncodeToString(teeInfoData.Hash())` | computed per request |
| `AllowedDebugStatuses` | `["disabled-since-boot"]` when `!AllowTeeDebug`, else empty (skips dbgstat check) | derived from `ALLOW_TEE_DEBUG` |

**What `ParseAndValidatePKIToken` enforces under this Policy** (in order):

| Check | Source | Notes |
|---|---|---|
| JWT signature | x5c leaf key | RS256 only (`jwt.WithValidMethods`). Other algs rejected. |
| x5c chain | embedded Google root | leaf → intermediate → root; root compared via `x509.Certificate.Equal`. |
| Leaf Extended Key Usage | leaf cert's own `ExtKeyUsage` | Chain verification pins `KeyUsages` to the leaf's declared EKUs (no longer the permissive `ExtKeyUsageAny`). `Policy.AllowedLeafEKUs` can override; verifier doesn't set it. |
| CRL revocation | leaf + intermediate CRLs | nil CRL → warn + skip (verifier doesn't set `Policy.RequireCRL`). See §7.1 CRL revocation checking. |
| `iss` claim | `Policy.Issuer` (defaults to `ConfidentialSpaceIssuer`) | always checked. |
| `exp` claim | token | `jwt.WithExpirationRequired()` — expiry must be present and unexpired. |
| `aud` claim | `Policy.Audience` (= `TEE_AUDIENCE`, or `DefaultTeeAudience`) | the audience the TEE requested its token for; rejects tokens issued for a different consumer. Always populated (config defaults it), so the empty-skip path is not reachable in normal operation. |
| Clock skew | `Policy.Leeway` (defaults to 30s) | applied to `exp`/`iat`/`nbf`. |
| `dbgstat` | `Policy.AllowedDebugStatuses` | `["disabled-since-boot"]` in production; empty when `ALLOW_TEE_DEBUG=true` (skipped). |
| `eat_nonce` | `Policy.EATNonce` (= `hex.EncodeToString(teeInfoData.Hash())`) | the token's `eat_nonce` must be **exactly one** entry equal to this value (see below). |

`image_id` is **not** enforced at the JWT layer: the verifier leaves `Policy.AllowedImageIDs` unset, so `ValidateClaims` skips the image_id check. The accepted workload code hash is enforced on-chain by the `ExtensionManager` (`isCodeHashPlatformSupported`).

EAT-nonce binding requires the token's `eat_nonce` to be **exactly one** entry equal to `Policy.EATNonce` (`hex.EncodeToString(teeInfoData.Hash())`) — a different count or value is rejected. tee-node requests its attestation token with a single nonce (the same teeInfo hash), so the token carries exactly one `eat_nonce`. Binding is enforced **by default (fail closed)**: it is only skipped when `Policy.SkipNonceCheck` is set, which the verifier never does — so an empty/missing nonce now fails verification rather than silently skipping (the pre-`ae050b4` go-flare-common did containment and skipped on an empty `EATNonce`).

**Policy fields the verifier currently does not set**: `RequireSecBoot` (could reject `secboot=false` Confidential Space VMs), `AllowedHWModels` (could restrict to e.g. `GCP_INTEL_TDX`), `AllowedLeafEKUs` (could pin to a specific EKU set), `RequireCRL` (would fail closed when a cert declares DPs but no CRL is supplied). Each defaults to "skip"; enabling them would tighten the surface further.

**Claims validation (`ValidateClaims`):** Runs only after the Policy has accepted the token. Covers what the Policy does not:
1. **Software name** — Must equal `"CONFIDENTIAL_SPACE"`.
2. **Stability** — Production path only (reached when the token's `dbgstat == "disabled-since-boot"`; only path possible when `AllowTeeDebug=false`). If `SupportAttributes` is nil → hard error (verification fails). If present but `"STABLE"` not in the list → returns status `OBSOLETE`. Debug-mode TEEs (admitted when `AllowTeeDebug=true` and the Policy's dbgstat allowlist is empty) skip this check and always return `OK`.
3. **CodeHash** — Extracted from `SubMods.Container.ImageID` (sha256 digest → 32-byte hash).
4. **Platform** — Extracted from `HWModel` claim (e.g. `"GCP_INTEL_TDX"` → 32-byte hash).

**Bypasses**:
- `DISABLE_ATTESTATION_CHECK_E2E=true` — skips JWT validation entirely (E2E only).
- **MagicPass** — **Cannot admit a rogue TEE on-chain in a normal production deployment.** The verifier accept path exists, but the on-chain confirmation rejects the resulting proof because none of the test code hash / test platform values are registered or whitelisted on mainnet. Detail:

  *Verifier behavior.* TEE nodes in non-production mode (`settings.Mode != 0`) return `"magic_pass"` instead of a real attestation token. The verifier unconditionally accepts it, skips all attestation validation (PKI, claims, CRL), and returns `OK` with hardcoded test values: `E2ETestCodeHash` and `E2ETestPlatform` (`"TEST_PLATFORM"` UTF-8 padded). Gated by the TEE node's `settings.Mode` — the verifier itself has no toggle.

  *Contract-side admission chain* (`flare-smart-contracts-v2`):
  - `contracts/tee/facets/VerificationFacet.sol#confirmAvailability` is the on-chain entry point for committing an availability proof.
  - `MachineManager.checkTeeMachineInProduction(teeId)` — only PRODUCTION-status TEEs are accepted (`contracts/tee/library/MachineManager.sol`).
  - `MachineManager.checkCodeHashPlatformSupported(extensionId, teeMachine.codeHash, teeMachine.platform)` → `ExtensionManager.isCodeHashPlatformSupported` — at confirm time the `(codeHash, platform)` pair must not be disabled (`codeHashPlatformDisabled`) and must be present in the extension's `codeHashToVersion[codeHash].platforms` set (`contracts/tee/library/ExtensionManager.sol`). (`systemSupportedPlatforms` is enforced when a TEE version is *added*, not during `confirmAvailability`.)
  - `Verification.verifyAvailabilityCheckProof` → `_validateResponseBody` requires `responseBody.codeHash == registered.codeHash` and `responseBody.platform == registered.platform` (`contracts/tee/library/Verification.sol`).

  For magic_pass to actually admit on-chain, governance would have to register a TEE with `codeHash = E2ETestCodeHash` AND `platform = E2ETestPlatform`, whitelist that pair in `systemSupportedPlatforms` and the extension's `platforms` set, and move that TEE to `PRODUCTION` status. Every step is on-chain and visible; the sequence is operationally absurd on mainnet. In normal production a misconfigured magic_pass response silently fails on-chain confirmation — wasting DP/relay work but never producing a valid admission. Operators must not register test code hashes or whitelist `TEST_PLATFORM` on production networks.

  Supports hackathon/dev environments only; do not rely on it in production.

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

### Request deadline & dependency timeouts
Verifier work runs under an authoritative per-request deadline (`verifierWorkTimeout`, 25s), applied in the handler around `Verify` and kept below the server `writeTimeout` (30s) so the verifier abandons a hung dependency before the HTTP write deadline. Because downstream calls run under that context — the DB repos use `WithContext` and the nonce binder uses the request ctx — the deadline actually cancels in-flight DB queries and RPC calls rather than leaking goroutines. A timed-out or cancelled verification surfaces `context.DeadlineExceeded`/`Canceled`, classified as **503**.

Defense-in-depth at the driver level bounds individual statements even if context cancellation does not promptly abort one: the Postgres source DB sets a true server-side per-session `statement_timeout` (via the pgx DSN, both URL and keyword forms) and the MySQL C-chain DB sets read/write I/O timeouts (`readTimeout`/`writeTimeout`; go-sql-driver has no portable server-side statement cap), both at `dbStatementTimeout` (28s — above the 25s request deadline so they never pre-empt a legitimately in-progress query, and below the 30s `writeTimeout` so a backstop abort still leaves margin to write a 503). The on-chain `getInitialNonce` call additionally has its own short `rpcTimeout` (5s).

### CRL revocation checking
Intermediate + leaf certs from the x5c chain are checked for revocation.

**Validation** (in `go-flare-common`, `pkg/tee/attestation/googlecloud/google_cloud.go`): `ParseAndValidatePKIToken(attestationToken, rootCert, leafCRL, intermediateCRL, policy)` accepts pre-fetched CRLs (nil when unavailable) and a `Policy` (see §7.1 JWT validation). `PKICertificates.Verify()` calls `verifyCRL()` after chain/lifetime checks; per cert (leaf against intermediate, intermediate against root): if CRL nil → log + skip; else validate time window (`ThisUpdate` ≤ now ≤ `NextUpdate`), verify CRL signature (`CheckSignatureFrom(issuer)`), reject if serial in `RevokedCertificateEntries`.

**Fetching and caching** (`verifier/crl_cache.go`):
- Request-driven. `CRLCache.FetchCRLsForToken()` runs inline with request `ctx` before `ParseAndValidatePKIToken`.
- **Strict all-or-nothing**: if all CRL distribution points fail for either cert, verification fails.
- Parses token unverified (`ParsePKITokenUnverified`) to extract x5c. Before any CRL URL is dereferenced:
  - **Root match**: token's root must equal the embedded Google root.
  - **Chain pre-validation** (`validateX5CChain`): intermediate signed by root, leaf signed by intermediate (signature-only — revocation deferred to downstream `ParseAndValidatePKIToken`), plus `NotBefore`/`NotAfter` currency for all three certs. Rejecting bad chains here prevents attacker-supplied certs (with arbitrary CRL distribution point URLs) from triggering any outbound request.
- Reads `CRLDistributionPoints` from leaf + intermediate only after the chain is validated.
- Leaf + intermediate fetches run **in parallel**. For each cert, distribution points tried in order; first successful fetch used. `CheckSignatureFrom(issuer)` is verified before caching — CRL signed by a different CA is rejected and the next DP is tried.
- **Singleflight** (`singleflight.Group`) deduplicates concurrent fetches for the same URL.
- **Cache** (`sync.RWMutex`, keyed by URL): an entry is fresh iff all of (a) age < `crlMaxCacheTTL` (4h), (b) `NextUpdate` non-zero, (c) `NextUpdate` not passed. Zero `NextUpdate` → always re-fetch. TTL cap guards against emergency revocation before the old `NextUpdate`.
- On miss/stale, fetched via `fetchCRLBytes`: `ResolveExternalURL(ctx, url, false)` first (always rejects private/local addresses regardless of `ALLOW_PRIVATE_NETWORKS`, which is scoped to the TEE proxy), then `fetcher.FetchBytesPinned` with the resolved IP pinned to prevent DNS rebinding (2s timeout, redirects rejected). PEM-decoded if PEM (Google Cloud CRL endpoints return PEM), else raw DER; parsed with `x509.ParseRevocationList`.
- Eviction: at `crlMaxEntries` (100), stale entries purged; if still full, oldest evicted.
- `CRLCache.Close()` added to shutdown closers.
- Google CA Service only inserts the CDP extension when `publish_crl` is enabled (per-CA-pool setting). Currently the intermediate cert has a CDP but the leaf does not (no OCSP either). Google does not document CRL/OCSP checking for Confidential Space — the sample PKI token validation code only covers chain verification, root pinning, and signature checks; revocation checking must tolerate missing CDPs. See Google CA Service and Confidential Space PKI documentation for details.

### TEE status semantics
- Verification response status values: `0 = OK`, `1 = OBSOLETE`. Live-fetch failures surface per the error model (§9): TEE-proxy HTTP/non-OK (`ErrHTTPFetch`) and not-yet-available action results (`ErrActionResultNotFound`, 404) map to **503**; only ambiguous URL-validation / JSON-decode errors fall through to 500.
- Internal classification (used by `CheckSigningPolicies`): `TeeSampleValid`, `TeeSampleInvalid`, `TeeSampleIndeterminate`.

## 7.2 PMWPaymentStatus

### Primary flow (`XRPVerifier.Verify`)
1. Build instruction ID from `(opType, PAY, sourceID, senderAddress, paymentId, reissueNumber=0)` using ABI packing + keccak. PAY events use reissue number 0 (reissues start at 1).
2. Resolve `TeeInstructionsSent` event signature.
3. Fetch matching event log from C-chain index DB (`topic0`, `topic1=0`, `topic2=instructionID`).
4. Decode tee instruction message payload, binding the event back to the request against all instruction-ID inputs. The decoder (`DecodeTeeInstructionsSentEventData`) first checks the event wrapper's `OpType == req.OpType` and `OpCommand == PAY` (the PAY/REISSUE message schema alone does not prove the op), then `db.CheckInstructionConsistency` checks the decoded message's `SourceId`/`SenderAddress`/`PaymentId` equal the values the instruction ID was built from. The `topic2 = keccak(opType, op, sourceId, account, paymentId, reissueNumber)` query already commits to all of these, so any mismatch means the indexed event data disagrees with its own topic — a C-chain index inconsistency (`ErrDatabase` → 503). (The message's `Nonce` is not bound here: it is not part of the instruction ID — it carries the XRP sequence used to locate the transaction, bound separately by the row-consistency check below.)
5. **Sequence binding** (`pmwnonce.Binder.VerifySequence`): the decoded message's `Nonce` (the XRP sequence) must equal the value the contract deterministically assigns this payment — `initialNonce + paymentId - 1`, where `initialNonce` is read on-chain via `getInitialNonce(account)` on the source's per-source `TeePayments` contract (`TEE_PAYMENTS_CONTRACT_ADDRESS`, **not** the FlareTeeManager diamond) — the account's XRP sequence captured at registration, immutable afterwards, so cached per `(sourceId, account)`. The instruction-ID topic does not commit to `Nonce`, so without this a compromised indexer could point a `paymentId` at a foreign sequence (and thus a foreign transaction). A mismatch, or any failure reading `initialNonce`, fails closed (`ErrDatabase` → 503).
6. Query source DB transaction by `(source_address, sequence)`, where the sequence is the decoded message's `Nonce` (the request no longer carries it).
7. Parse raw source-chain transaction JSON. Reject if `TransactionType != "Payment"` — non-payment types (e.g. `AccountSet`, `TrustSet`) at the same `(sourceAddress, sequence)` cannot produce a payment status attestation.
8. **Row-consistency check** (`db.CheckRowConsistency`): the identity fields parsed from the response JSON (`hash`, `Account`, `Sequence`) must match the row's indexed columns (`Hash`, `SourceAddress`, `Sequence`) before the response is trusted; a mismatch is treated as a DB inconsistency (`ErrDatabase` → 503). Response rows over `maxResponseSize` are rejected the same way.
9. Build FDC2 response:
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
- Empty `publicKeys` list and `threshold == 0` rejected → 400 (a zero signer quorum is never a valid multisig). Enforced in the verifier as well as via the JSON `min=1`/`gte=1` tags, so ABI requests cannot bypass it.

### Primary flow (`XRPVerifier.Verify`)
1. Call XRPL `account_info` with `ledger_index=validated`, `signer_lists=true`.
2. **Bind the response to the request**: `account_data.Account` must equal `req.AccountAddress` (both normalized X-address→classic) before any field is trusted. A misbehaving/compromised RPC returning another (validly-configured) account's data is rejected here, so it cannot yield `OK` for the requested account. Mismatch → validation failure.
3. Require the response is from a validated ledger (`validated == true`).
4. Resolve signer lists from response. XRPL API v1 (rippled) returns `signer_lists` inside `account_data`; API v2 and Clio return it at the `result` level — both layouts supported.
5. Validate signer list exists and matches provided pubkeys + threshold. Set-based comparison — duplicate `publicKeys` cannot mask extra on-chain signers.
6. Validate account flags: master key disabled; deposit auth disabled; destination tag requirement disabled; incoming XRP disallow disabled.
7. Validate no regular key set.
8. Success → `{status=OK, sequence}`; validation failure → `{status=ERROR, sequence=0}`.

### Public key handling
- Parsed and compressed secp256k1; converted to XRPL address for signer-set comparison.

## 7.4 PMWFeeProof
Fee reconciliation attestation for PMW protocols. Compares estimated fees (from C-chain events) with actual fees (from XRP transactions) across a paymentId batch.

### Request
- `opType`, `senderAddress`, `firstPaymentId` (inclusive), `batchCount` (number of consecutive paymentIds), `untilTimestamp` (Flare block timestamp cutoff for reissues). The response also returns `lastPaymentId` (= `firstPaymentId + batchCount - 1`).
- `batchCount` must be > 0 and capped at 200 (`MaxBatchRange`); 0, over the cap, or a range that overflows uint64 → 400.
- Reissue scan capped at 32 reissue events per payment (`MaxReissuesPerPayment`). The contract has no on-chain cap on reissue count (only a per-batch timing gate), so this is a defense-in-depth backstop against indexer pollution / pathological retry behavior. Realistic legitimate flows reissue 0–3 times per payment. Exceeding the cap → 400.

### Primary flow (`XRPVerifier.Verify`)
1. Validate `batchCount` (> 0, ≤ `MaxBatchRange`, no uint64 overflow of `firstPaymentId + batchCount - 1`).
2. Compute pay instruction IDs for paymentIds `[firstPaymentId, firstPaymentId+batchCount)`; batch fetch C-chain events (`topic2 IN (?)`).
3. Per paymentId: verify pay event exists, extract `maxFee`, and read the XRP sequence from the decoded message's `Nonce`. Each decoded pay and reissue event is bound back to the request against all instruction-ID inputs: the decoder checks the wrapper `OpType == req.OpType` and `OpCommand == PAY`/`REISSUE`, and `db.CheckInstructionConsistency` checks the message's `SourceId`/`SenderAddress`/`PaymentId`; a mismatch is a C-chain index inconsistency (`ErrDatabase` → 503). The pay event's `Nonce` is additionally bound on-chain (`pmwnonce.Binder.VerifySequence`): it must equal `initialNonce + paymentId - 1` from `TeePayments.getInitialNonce(account)` (cached per account), so a compromised indexer cannot substitute a foreign sequence — and thus a foreign transaction's fee — for a paymentId; a mismatch or RPC failure → `ErrDatabase`. Fee magnitudes are not covered by the instruction-ID or row-consistency bindings, so impossible values fail closed: any single pay/reissue `maxFee` or XRP `Fee` above the total XRP supply (`helper.MaxXRPDrops` = 1e17 drops), and any `estimatedFee`/`actualFee` total exceeding `uint256`, → `ErrDatabase`.
4. Per paymentId: iteratively fetch reissue events (reissueNumber 1, 2, 3... — PAY is number 0, reissues start at 1 — until not found, `blockTimestamp > untilTimestamp`, or past `MaxReissuesPerPayment`). If the loop hits the cap and `MaxReissuesPerPayment+1` exists in the indexer **within the same `untilTimestamp` window** (a probe event past `untilTimestamp` is outside this batch and does not count), return `ErrReissueLimitExceeded`. Otherwise add residual `max(0, reissue_maxFee - pay_maxFee)` for each scanned reissue.
5. Sum as `estimatedFee`.
6. Batch fetch XRP transactions by the sequences read in step 3 (`sequence IN (?)`), parse `Fee`, sum as `actualFee`. Before trusting each `Fee`, the row passes the same consistency check as PMWPaymentStatus (`db.CheckRowConsistency`: parsed JSON `hash`/`Account`/`Sequence` must equal the row's columns) and a `maxResponseSize` cap; a mismatch or oversize row is a DB inconsistency (`ErrDatabase` → 503).
7. Return `{lastPaymentId, actualFee, estimatedFee}`.

### Error handling
- Missing pay event for any paymentId → 422 (`ErrMissingPayEvent`).
- Missing XRP transaction for any paymentId → 422 (`ErrMissingTransaction`).
- Invalid batch range (zero, too large, or overflow) → 400 (`ErrBatchRangeTooLarge`).
- Reissue scan exceeded the per-payment cap → 400 (`ErrReissueLimitExceeded`).
- DB infrastructure failure, DB-row inconsistency (parsed JSON identity fields ≠ indexed columns), or oversize response row → 503 (via `ErrDatabase`).

### Data retention
Both PMWPaymentStatus and PMWFeeProof read transaction/event data entirely from indexer databases; the only chain/RPC dependency is the read-only `TeePayments.getInitialNonce` call used for the sequence binding (`RPC_URL` is a required config var for both, and `initialNonce` is cached per account in a bounded LRU since it is immutable post-registration). The XRP indexer retains transaction data for a configurable period (typically ~2 weeks in production); the C-chain indexer has its own retention policy. Requests outside retention → 422 for missing data. FDC2 attestation requests are tied to reward epochs with short deadlines, so out-of-retention requests indicate a protocol-level delay, not normal operation.

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
  - batch range invalid (zero, too large, or overflow) — `ErrBatchRangeTooLarge` (PMWFeeProof)
  - reissue scan exceeded `MaxReissuesPerPayment` — `ErrReissueLimitExceeded` (PMWFeeProof)
- `401 Unauthorized`:
  - missing/invalid `X-API-KEY` (except `/api/health`)
- `422 Unprocessable Entity`:
  - XRP RPC returned non-success status (e.g., account not found) — `ErrRPCNonSuccess` (PMWMultisig)
  - requested record not found in database (instruction log or transaction) — `ErrRecordNotFound` (PMWPaymentStatus)
  - missing pay event for paymentId — `ErrMissingPayEvent` (PMWFeeProof)
  - missing XRP transaction for paymentId — `ErrMissingTransaction` (PMWFeeProof)
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
  - database infrastructure failure (connection, timeout); DB-row inconsistency (`CheckRowConsistency`: parsed JSON `hash`/`account`/`sequence` ≠ indexed columns); C-chain event-data inconsistency (decoder `OpType`/`OpCommand` and `CheckInstructionConsistency` `SourceId`/`SenderAddress`/`PaymentId` ≠ the instruction ID inputs); or response row exceeding `maxResponseSize` — `ErrDatabase` (PMWPaymentStatus, PMWFeeProof)
  - network errors from RPC calls — `ErrNetwork` (TEE)
  - RPC server-side errors — `ErrRPC` (TEE)
  - context deadline/canceled — `ErrContext` (TEE)
  - unclassified RPC errors (indeterminate → retry) — `ErrUnknown` (TEE)
  - HTTP request or non-OK status from TEE proxy — `ErrHTTPFetch` (TEE)
  - TEE action/result returned 404 (result not yet available in Redis) — `ErrActionResultNotFound` (TEE)
  - verifier work exceeded the per-request deadline, or the request was cancelled — `context.DeadlineExceeded`/`context.Canceled` (all attestation types)

Notes: PMWMultisig's `500` default branch is defensive and not reachable under normal operation. PMWMultisig validation failures (wrong signers, wrong flags, etc.) return a `200` with `status=ERROR`, not an HTTP error.

## 10. Concurrency and State
- **Parallelism**: TEE `Verify` runs `DataVerification` + `CheckSigningPolicies` concurrently after the challenge fetch; `CheckSigningPolicies` fetches initial + last signing policy hashes concurrently; CRL leaf + intermediate fetches run concurrently inside `FetchCRLsForToken`.
- **Caches**: CRL cache uses `sync.RWMutex` (RLock fast path for hits, WLock for inserts/eviction) + `singleflight.Group` to dedupe concurrent fetches for the same URL.
- **Config loaders**: `sync.Once` singletons.

## 11. Testing Strategy in Repo
- Unit tests across API/config/attestation subpackages.
- Integration-style tests under `internal/tests/server`.
- Docker-based fixtures for payment-status deps (`internal/tests/docker/docker-compose.yaml`).
- `gencover.sh` orchestrates coverage + docker lifecycle.
- TEE availability server tests set `ALLOW_PRIVATE_NETWORKS=true` to allow `httptest` localhost URLs.

## 12. Operational Notes and Risks
- `PMWPaymentStatus` and `PMWFeeProof` key on `paymentId`, which is part of the instruction ID (`instructionId = keccak(opType, op, sourceId, accountAddress, paymentId, reissueNumber)`; PAY uses reissue number 0, reissues start at 1). Each `paymentId` therefore resolves to a unique instruction event. The XRP sequence used to locate the actual transaction is **not** in the request — it is read from the decoded event message's `Nonce` field, bound on-chain to `initialNonce + paymentId - 1` (`TeePayments.getInitialNonce`), and bound to its row by `CheckRowConsistency`. `paymentId` is a per-account sequential identifier (1-based); the requester (e.g. FAsset) maps its logical payment id to the on-chain `paymentId`. For the current XRP/account-only scope one `paymentId` maps to one payment and one XRP transaction; if multiple events ever shared an instruction ID, `FetchInstructionLog` fails closed (`ErrDatabase`) rather than picking one. UTXO sources additionally key on an `anchorIndex` (not yet implemented).

### Accepted risks
- **MagicPass bypass** (`verifier.go`): cannot admit a rogue TEE on-chain in a normal production deployment.
  - *What it is*: TEE nodes in non-production mode return `"magic_pass"`; the verifier accepts it and returns `OK` with `E2ETestCodeHash` / `E2ETestPlatform`. No verifier-side toggle; gated by the TEE's `settings.Mode`.
  - *Compensating controls*:
    1. Production TEE nodes never set `Mode != 0`.
    2. On-chain confirmation rejects the proof unless the registered TEE's `codeHash`/`platform` match the response — see the chain documented in §7.1 (`VerificationFacet.confirmAvailability` → `Verification.verifyAvailabilityCheckProof` → `_validateResponseBody` + `MachineManager.checkCodeHashPlatformSupported`).
  - *Residual risk*: a misconfigured magic_pass response on mainnet wastes DP/relay work and surfaces as a failed on-chain confirmation; it does not produce a valid admission. Operators must not register the test code hash or whitelist `TEST_PLATFORM` on production networks.
- **Unauthenticated Swagger UI** (`/api-doc`): The OpenAPI documentation endpoint is intentionally exempt from API key auth to allow internal developers and auditors to browse the API. Compensating control: service is deployed behind internal infrastructure, not exposed to the public internet. No sensitive data is served on this endpoint.
- **HTTP redirects disabled** (`fetcher.go`): HTTP clients reject all redirects (`CheckRedirect` returns `ErrRedirect`). TEE proxy URLs are expected to resolve directly — TEE nodes cannot follow redirects on their POST-based proxy communication, so operators already configure non-redirecting URLs. Eliminates the SSRF bypass vector where a redirect target could point to a private/metadata IP.
- **CRL fetch SSRF defenses** (`crl_cache.go`): CRL distribution points come from x5c certs inside an unverified JWT, so two layers gate the fetch: (1) x5c chain pre-validation (intermediate signed by root, leaf signed by intermediate, validity windows current) before any URL is dereferenced — attacker-supplied cert chains never trigger outbound requests; (2) `ResolveExternalURL(ctx, url, false)` with `allowPrivateNetworks=false` hardcoded (CRLs must never resolve to private/local addresses; `ALLOW_PRIVATE_NETWORKS` is scoped to the TEE proxy and is not honored here), followed by `fetcher.FetchBytesPinned` so the connection is pinned to the resolved IP.
- **ABI event data decoding** (`instruction_event.go`): `DecodeTeeInstructionsSentEventData` rejects `log.Data` larger than 1 MB (`maxEventDataSize`) before ABI decoding. Legitimate events are ~1–2 KB; the cap prevents OOM from corrupted indexer data.

## 13. Minimal Runtime Sequences
**Start**: load env → validate common config → build module-specific config → build verifier/service dependencies → register endpoints + auth middleware → start HTTP server.

**Shutdown**: receive OS signal → HTTP graceful shutdown (`10s`) → close module resources (DB, eth client, CRL cache).
