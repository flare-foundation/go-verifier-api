# Verifier Conformance Spec — Scoping

Purpose: define a language-neutral behavioral spec + test vector suite for the four verifier types in this repo, so any implementation (Rust, Python, TypeScript, …) that passes the suite is provably conformant.

Primary motivation: mitigate single-codebase monoculture risk for the TEE attestation verifier. The other three verifier types (PMW*) share the same monoculture property and are included in the same document since the HTTP API contract, error model, and test infrastructure are shared.

Source of truth today: `docs/SPEC.md` + Go test files in `internal/attestation/`.

---

## 1. Verifier types in scope

| Type | Role | Trust criticality | Spec depth |
|---|---|---|---|
| `TeeAvailabilityCheck` | TEE admission control (JWT attestation) | **Critical** — admits TEEs into signing policy | Deepest section, full JWT/PKI/CRL coverage |
| `PMWPaymentStatus` | XRP payment status attestation | Payment-bound | Medium — DB schema, ABI decoding, raw tx parsing |
| `PMWMultisigAccountConfigured` | XRPL multisig config attestation | Wallet-bound | Medium — XRPL RPC, signer-list comparison |
| `PMWFeeProof` | XRP fee reconciliation attestation | Fee-bound | Medium — event aggregation, paymentId batch scanning |

---

## 2. Deliverables

### 2.1 Behavioral spec document
A prose document describing what each verifier does, with no Go-specific references. Language-neutral.

**Shared sections (apply to all four):**
- **A — HTTP API surface**: request/response shapes, status codes, headers, auth model, request-size limit
- **B — ABI/encoding contract**: how requests/responses are encoded, ID-equality enforcement
- **C — Error taxonomy**: error categories + HTTP status mapping, no language-specific error wrapping
- **D — Auth model**: API key header, exempt routes (`/api/health`, `/api-doc`)
- **E — Security headers**: `X-Frame-Options`, `X-Content-Type-Options`
- **F — Error sanitization rules**: generic message in response, full detail in server log, request ID correlation

**Per-verifier-type sections:**

**TeeAvailabilityCheck** (deepest):
- T1 — URL validation pipeline (scheme, userinfo, localhost, IP categories, DNS pinning, `ALLOW_PRIVATE_NETWORKS` toggle)
- T2 — Challenge fetch, challenge equality check, proxy signer recovery
- T3 — JWT PKI validation (x5c chain, root pinning)
- T4 — Claim validation (EATNonce, dbgstat, swname, support_attributes, image_id, hwmodel)
- T5 — CRL handling (fetch, parse, verify, cache, staleness, all-or-nothing semantics)
- T6 — Signing policy check (relay contract hashes, initial + last, parallel RPC)
- T7 — Bypasses (`DISABLE_ATTESTATION_CHECK_E2E`, `ALLOW_TEE_DEBUG`, `magic_pass`)
- T8 — TEE status semantics (`OK` / `OBSOLETE`)

**PMWPaymentStatus**:
- P1 — Instruction ID build (ABI pack + keccak of `opType, PAY, sourceID, accountAddress, paymentId, reissueNumber=0`)
- P2 — C-chain event lookup (`topic0=signature, topic1=0, topic2=instructionID`)
- P3 — Instruction message decoding
- P4 — Source DB query (`source_address, sequence`)
- P5 — Raw tx parsing (must be `Payment`; non-payment types rejected)
- P6 — Response composition (recipient/amount from instruction; received amount from `AffectedNodes` AccountRoot balance changes; X-address normalization)

**PMWMultisigAccountConfigured**:
- M1 — Request validation (publicKeys capped at 32, no empty entries, no duplicates)
- M2 — XRPL `account_info` call (`ledger_index=validated`, `signer_lists=true`)
- M3 — SignerList resolution across XRPL v1 vs v2/Clio response shapes
- M4 — Set-based signer comparison (duplicates cannot mask extras)
- M5 — Account flag checks (master disabled, deposit auth, dest tag, disallow XRP)
- M6 — Regular-key absence check
- M7 — Public key handling (secp256k1 compression, XRPL address derivation)
- M8 — Result composition (`{status, sequence}`)

**PMWFeeProof**:
- F1 — Batch range validation (`batchCount`, cap = 200)
- F2 — Pay instruction ID batch computation
- F3 — Pay event batch fetch from C-chain
- F4 — Reissue event iteration (per payment, reissueNumber from 1, until not found or timestamp cutoff)
- F5 — Residual fee computation (`max(0, reissue_maxFee - pay_maxFee)`)
- F6 — XRP transaction batch fetch
- F7 — Actual fee summation
- F8 — Result composition (`{actualFee, estimatedFee}`)

### 2.2 Test vector suite
JSON test vectors that any implementation must process to the same output. One folder per verifier type.

```
docs/conformance/
  README.md
  shared/
    api_contract.json          # HTTP request/response shape tests
    error_mapping.json         # error class → HTTP code tests
    auth.json                  # API key auth tests
  tee-availability-check/
    fixtures/
      google_root_cert.pem
      sample_jwts/...
      teeinfo/...
      crls/...
    test-vectors/
      claims_validation.json
      crl_revocation.json
      url_validation.json
      bypass_flags.json
      eat_nonce_binding.json
      signing_policy.json
  pmw-payment-status/
    fixtures/
      chain_logs/...           # canned topic0/1/2 event logs
      raw_xrp_tx/...           # raw XRPL transaction JSON
      instruction_messages/...
    test-vectors/
      instruction_id.json
      payment_decoding.json
      non_payment_rejection.json
      received_amount.json
      x_address_normalization.json
  pmw-multisig-account-configured/
    fixtures/
      account_info_v1/...
      account_info_v2_clio/...
      public_keys/...
    test-vectors/
      signer_list_v1_v2_resolution.json
      flag_validation.json
      regular_key_absence.json
      signer_set_comparison.json
      public_key_cap.json
  pmw-fee-proof/
    fixtures/
      pay_events/...
      reissue_events/...
      xrp_transactions/...
    test-vectors/
      nonce_range_cap.json
      residual_fee.json
      reissue_iteration.json
      timestamp_cutoff.json
      fee_summation.json
```

**Test case format:**
```json
{
  "name": "rejects debug TEE when ALLOW_TEE_DEBUG=false",
  "verifier": "TeeAvailabilityCheck",
  "input": {
    "jwt": "fixtures/sample_jwts/valid_debug.jwt",
    "teeInfo": "fixtures/teeinfo/sample_0.json",
    "rootCert": "fixtures/google_root_cert.pem",
    "config": { "allowTeeDebug": false, "disableAttestationCheckE2E": false }
  },
  "expected": {
    "result": "reject",
    "errorContains": "TEE is not running in production mode"
  }
}
```

### 2.3 Reference test runner
A small script (Go for parity, Python for portability — pick one) that loads test vectors and exercises a verifier under test. Vendor-neutral. Should support running against a binary (e.g. by spawning it and sending HTTP) or a library (in-process).

---

## 3. Test category coverage

### TeeAvailabilityCheck
| # | Category | Coverage |
|---|---|---|
| 1 | EATNonce binding | nonce count != 1, nonce mismatch, valid binding |
| 2 | Debug status | production / debug, with/without `ALLOW_TEE_DEBUG` |
| 3 | SWName | accept `CONFIDENTIAL_SPACE`, reject anything else |
| 4 | Support attributes | nil → error, `[STABLE]` → OK, `[]` → OBSOLETE |
| 5 | CodeHash extraction | valid sha256, malformed digest |
| 6 | Platform extraction | valid `HWModel`, malformed value |
| 7 | x5c chain | valid chain, missing intermediate, wrong root |
| 8 | JWT signature | valid, tampered payload, wrong key |
| 9 | CRL fetch | DP reachable, all DPs fail, signature mismatch |
| 10 | CRL revocation | cert in revoked list vs not |
| 11 | CRL staleness | `NextUpdate` passed, 4h TTL expired, both fresh |
| 12 | URL validation | scheme, userinfo, private IPs (strict + permissive), DNS pinning |
| 13 | `magic_pass` bypass | TEE returns `"magic_pass"` → OK with test values |
| 14 | `DISABLE_ATTESTATION_CHECK_E2E` | bypass behavior |
| 15 | Challenge freshness | block timestamp within threshold vs stale |
| 16 | Signing policy | matches relay contract vs mismatch |
| 17 | Error → HTTP code mapping | each error class maps to correct status |

### PMWPaymentStatus
| # | Category | Coverage |
|---|---|---|
| 1 | Instruction ID derivation | exact ABI pack + keccak match for known inputs |
| 2 | C-chain log lookup | topic match found / missing → 422 |
| 3 | DB query semantics | source_address + sequence match |
| 4 | Non-payment rejection | `AccountSet`, `TrustSet` → reject |
| 5 | Received amount | computed from `AffectedNodes` for OK and reverted txs |
| 6 | X-address normalization | X-address input vs classic address match |
| 7 | Error → HTTP code mapping | not-found, ABI decode, DB failure |

### PMWMultisigAccountConfigured
| # | Category | Coverage |
|---|---|---|
| 1 | publicKeys cap | 32 ok, 33 → 400 |
| 2 | Empty publicKeys | empty entries → 400 |
| 3 | Signer list resolution | v1 response shape vs v2/Clio shape |
| 4 | Set-based comparison | duplicate publicKeys, extra on-chain signers |
| 5 | Account flags | each flag combination → expected outcome |
| 6 | Regular key absence | set → reject; not set → accept |
| 7 | Public key parsing | valid compressed, malformed, address derivation |
| 8 | RPC failure | network error → 503, non-success → 422 |

### PMWFeeProof
| # | Category | Coverage |
|---|---|---|
| 1 | Batch range cap | batchCount 200 ok, 201 → 400 |
| 2 | Missing pay event | any paymentId missing → 422 |
| 3 | Missing XRP tx | any paymentId missing → 422 |
| 4 | Reissue iteration | 1/2/3/... terminating on not-found or timestamp |
| 5 | Residual fee math | `max(0, reissue_maxFee - pay_maxFee)` exact |
| 6 | Fee summation | actual + estimated cross-checked against fixtures |
| 7 | Timestamp cutoff | `untilTimestamp` boundary inclusive/exclusive |
| 8 | DB infrastructure failure | → 503 |

---

## 4. Fixture generation — effort breakdown

### Easy (purely synthetic)
- TEE info blobs (already exist in `tmp/tee_info_response_0.txt`)
- `HWModel` test strings, container image_id strings, URL test cases
- Instruction message bytes (synthesize via ABI pack)
- XRPL public keys (test secp256k1 keys)
- Raw XRPL transaction JSON (handcrafted, no signature needed for verifier tests)
- C-chain event logs (synthesize topic0/1/2 + data)
- XRPL `account_info` response JSON (both v1 and v2/Clio shapes)

### Medium (need test crypto)
- Mock JWTs signed with a test CA — spec implementations use this CA's root as `googleRootCertificate`. Existing Go tests already do this; extract as fixtures.
- Mock CRLs signed by the test CA (revoked + unrevoked, fresh + stale)
- Test certificate chain (root + intermediate + leaf)

### Hard (need real Google attestation material)
- A real Google Confidential Space JWT for the "real-world happy path" test. Captured from a live TEE; has expiry, so the fixture must either:
  - Use a stable historical JWT + a mocked clock during test
  - Use a recently-captured JWT, refreshed periodically by CI
- Real Google CRL fixtures matched to the real JWT's x5c

Smallest in count, highest effort. Reasonable to require only synthetic fixtures for conformance, and treat real-Google validation as a separate integration gate.

---

## 5. Pluggable boundaries the spec must call out

For test-vector replay to work across implementations, these must be mockable in any conformant implementation:

- **HTTP fetcher** — for TEE proxy, CRL distribution points, XRPL RPC. Replays canned responses.
- **Clock** — for CRL staleness, challenge freshness, JWT expiry, timestamp cutoffs in fee proofs.
- **Chain RPC client** — for signing policy hashes and block freshness checks.
- **DNS resolver** — for URL validation tests.
- **Database** — for PMWPaymentStatus and PMWFeeProof, must accept a swappable DB driver pointing at fixture data.

The spec should state: "implementations MUST provide a way to inject these for testing." The Go implementation already does this.

---

## 6. What's NOT in scope

- Performance / load testing — separate concern
- Wire-level HTTP fuzzing — separate concern
- Replication of `go-flare-common`'s ABI encoding internals — that lives in a shared library, not the verifier
- DB schema migrations — implementations choose their own DB stack

---

## 7. Suggested phasing

| Phase | Work | Effort |
|---|---|---|
| 1 | Write `docs/CONFORMANCE_SPEC.md` (shared sections A–F + per-type spec sections) | 4–6 days |
| 2 | Synthetic fixtures + test vectors for all four types, all categories except real-Google | 2–3 weeks |
| 3 | Real-Google JWT + CRL fixtures (TeeAvailabilityCheck only) | 3–5 days + ongoing refresh |
| 4 | Reference test runner (Python or Go) | 3–5 days |
| 5 | Wire Go verifier to the suite; prove it passes | 2–3 days |

Phases 1, 2, 4, 5 are sufficient to define conformance. Phase 3 strengthens it but is optional for v1.

---

## 8. Open questions before starting

- Where does the spec live — `docs/CONFORMANCE_SPEC.md` here, or a separate repo?
- Does the test runner ship as a Go binary, a Python script, or both?
- Are real-Google fixtures (Phase 3) blocking for v1, or shippable later?
- Who owns refreshing real-Google fixtures when JWTs expire?
- Is there budget/intent to actually fund a Rust (or other) implementation, or is this purely about making one *possible*?
- For PMWPaymentStatus / PMWFeeProof, do alternative implementations have to use the same DB schemas, or just produce the same outputs from equivalent fixture data?
