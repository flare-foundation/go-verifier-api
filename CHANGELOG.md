# Changelog

All notable changes to this project are documented in this file. Versions follow
[Semantic Versioning](https://semver.org); dates are tag dates.

## [Unreleased] — v0.1.0

### Breaking: one deployment per source

- A deployment is now selected by `SOURCE_ID` alone and serves every attestation
  type its source offers: the XRP deployment serves PMWMultisigAccountConfigured,
  PMWPaymentStatus and PMWFeeProof; the TEE deployment serves TeeAvailabilityCheck.
  `VERIFIER_TYPE` no longer exists — env files from v0.0.2 will not boot.
- `RPC_URL` is split by role: `FLARE_RPC_URL` is the Flare C-chain node
  (PMWPaymentStatus/PMWFeeProof `getInitialNonce`, TeeAvailabilityCheck Relay
  lookups) and `SOURCE_RPC_URL` is the source-chain node (the XRPL node used by
  PMWMultisigAccountConfigured). A v0.0.2 PMWMultisigAccountConfigured deployment's
  old `RPC_URL` maps to `SOURCE_RPC_URL`; every other deployment's maps to
  `FLARE_RPC_URL`.
- Per-network templates (`.env.example`, `.env.coston`, `.env.coston2`, new
  `.env.songbird`) are rewritten as commented source profiles; exactly one profile
  is uncommented per deployment.

### Added

- **Relay cutover** (optional): `RELAY_CUTOVER_CONTRACT_ADDRESS` and
  `RELAY_CUTOVER_STARTING_REWARD_EPOCH` route each signing-policy lookup to the
  Relay contract that owns the id — ids at or above the starting reward epoch go
  to the redeployed Relay, lower ids stay on `RELAY_CONTRACT_ADDRESS`. Set both or
  neither; the epoch is a non-zero uint24 and must equal the
  `[relay_cutover] starting_reward_epoch` configured in the other Flare clients.
  No wall-clock switching, no fallback between contracts, no startup probe of the
  next Relay (deploy the pair ahead of the switch), no restart at the boundary.
  With the pair unset, Relay lookup behavior is identical to v0.0.2.
- **Status-based `/verify` envelope** for tee-relay-client: responses carry a
  VERIFIED/REJECTED/RETRY status with granular rejection reasons instead of lumped
  errors. Retryable and terminal failures are classified consistently across the
  HTTP path and the envelope for all attestation types — transient CRL fetch
  failures (timeouts, 5xx, 408/429, temporary DNS, body-read drops) and unusable
  indexer data map to RETRY/503; deterministic failures (404, other 4xx, refused
  redirects, oversized responses, bad URLs) reject terminally. An AST-based drift
  guard keeps the two classifiers in parity.
- XRPL network pinning for PMWMultisigAccountConfigured: the verifier pins the
  XRPL node's `network_id` and fails closed on a mismatch.

### Fixed / hardened

- CRL cache entries are scoped to `(URL, issuer certificate)` and every cached or
  fetched CRL is verified against its issuer (issuer-name binding plus signature),
  closing a cache-poisoning avenue between issuers sharing a distribution URL
  (audit finding 3.15).
- CRLs without a `NextUpdate` are rejected outright and never cached.
- Deprecated site-local IPv6 (`fec0::/10`) is blocked in TEE-proxy URL validation —
  it counted as public, allowing SSRF into networks that still route it (audit
  finding 3.29).
- Oversized TEE-proxy responses are rejected rather than truncated, and
  proxy-response validation failures are classified as 422.
- Env templates ship every source profile commented out and warn that exactly one
  may be uncommented — two active profiles silently use whichever comes last.

### Dependencies

- Go toolchain and build image to 1.26.6 (patched); `golang.org/x/text` v0.39.0
  (GO-2026-5970); `go-flare-common` updated.

### Notes

- BTC attestation support (PMWMultisigUtxoConfigured, BTC PMWPaymentStatus) was
  developed in this range and then moved off the release line: this release serves
  XRP and TEE sources only. The BTC line continues on `develop-btc`.

## [v0.0.2] — 2026-08-04

- Defense-in-depth hardening from the security-audit review: fee-magnitude and
  decimal-length bounds in PMWFeeProof, CRL cache guards, additional URL-validation
  blocks.
- PMWMultisigAccountConfigured rejects duplicate signer keys (XRP parity).
- Dependency patches for reachable CVEs: `pgx` v5.9.2, Go 1.26.4 toolchain.

## [v0.0.1] — 2026-07-29

Initial release. FDC2 verifier API with one deployment per attestation type
(`VERIFIER_TYPE`), serving:

- **TeeAvailabilityCheck** — Google Confidential Space attestation verification
  (PKI, claims, CRL revocation with cached fetches, SSRF-guarded URL validation,
  DNS pinning), Relay signing-policy checks, chain-ID pinning of attested TEEs.
- **PMWMultisigAccountConfigured, PMWPaymentStatus, PMWFeeProof** (XRP) — protocol-
  managed-wallet attestations backed by the C-chain and XRP indexer databases,
  with fail-closed bounds on indexer data, on-chain `initialNonce` binding, and
  reissue-window fee accounting.

API-key authentication (minimum key length enforced at boot), request deadlines
and DB timeouts, and CI workflows.
