# PMWFeeProof

New attestation type for proper fee accounting for protocols using PMW.

## Source
XRP only (same as PMWPaymentStatus).

## Request
- `opType` — needed to compute deterministic instruction IDs for event lookup (e.g. `F_XRP`)
- `senderAddress` — source chain sender address (XRP)
- `fromNonce`
- `toNonce`
- `untilTimestamp` — Flare chain block timestamp; defines the cutoff for fetching reissue events

## Response
- `actualFee` — in drops, sum of executed transaction fees across all nonces in range
- `estimatedFee` — in drops, sum of maxFees from pay and reissue events across all nonces in range

## Events
Both `pay` and `reissue` emit `TeeInstructionsSent`, differentiated by command (`PAY` vs `REISSUE`).
- Pay instruction ID: `keccak256(abi.encode(opType, PAY, sourceId, senderAddress, nonce))`
- Reissue instruction ID: `keccak256(abi.encode(opType, REISSUE, sourceId, senderAddress, nonce, reissueNumber))`
- Multiple reissues per nonce possible (tracked by `reissueCounter`).

## Logic
1. Find all pay and reissue events for `fromNonce` to `toNonce`. Check for reissues until `untilTimestamp`.
2. For each nonce: `estimatedFee += pay_maxFee + sum(max(0, reissue_N_maxFee - pay_maxFee))` — add pay maxFee plus the residual of each reissue (clamped to 0 if negative).
3. Check executed transactions and sum their fees as `actualFee`.

> **Note:** `estimatedFee` formula needs to be discussed with the Fasset team — confirm if this is useful as-is or if a different calculation would be more appropriate.

## Error handling
- Every nonce in range must have a pay event; missing pay event → error (422).
- Missing XRP transactions for any nonce in range → error (422, consistent with PMWPaymentStatus `ErrRecordNotFound`).
- Reissues after `untilTimestamp` are not the verifier's concern. Caller must first use PMWPaymentStatus to confirm `toNonce` is complete, then request PMWFeeProof.
- Follow PMWPaymentStatus pattern: 422 for missing data, 503 for DB failures, 500 for data corruption.

## Data retention
- XRP indexer retention is configurable (`history_drop` in indexer config), typically ~2 weeks in production. Callers must request PMWFeeProof within this window or the XRP transactions will no longer be available.

## Nonce range cap
- Cap `toNonce - fromNonce` to prevent heavy queries (e.g. max 100).
- Enforce at handler level (return 400 if exceeded).
- Hardcoded constant (convention in the codebase). Actual value TBD — benchmark once queries exist.

---

## Open questions
- **Nonce range cap value** — suggested ~100, needs benchmarking.
- **Error messages** — define distinct error messages for: missing pay event for nonce, missing XRP transaction for nonce, nonce range partially indexed. 503 is the only retryable status; 422 errors should include which nonce(s) failed.
- **Data retention** — XRP indexer holds ~2 weeks of data. Should the verifier validate that the requested range falls within the retention window, or just let it fail with 422 if data is missing?

## Implementation notes

### Architecture
Standalone deployment, same pattern as PMWPaymentStatus (own LoadModule entry, config, service/verifier).
Reuses same data sources (Postgres source DB + MySQL C-chain index DB), can point to the same instances.
ABI structs (`IPMWPMWFeeProofRequestBody` / `ResponseBody`) must preexist in `go-flare-common` and in the smart contracts.

### DB query strategy
- **Pay events**: instruction IDs are deterministic (`keccak(opType, PAY, sourceId, senderAddress, nonce)`). Compute all IDs for the nonce range and batch fetch with a new repo method: `WHERE topic0 = ? AND topic1 = ? AND topic2 IN (?)` (up to ~100 IDs).
- **Reissue events**: instruction IDs include `reissueNumber` which is unknown upfront. Query iteratively per nonce (reissueNumber 0, 1, 2... until not found). Reissues are rare, so most nonces won't need this.
- **XRP transactions** (for `actualFee`): batch fetch by sender and nonce range with a new repo method: `WHERE source_address = ? AND sequence IN (?)` (up to ~100 nonces).
