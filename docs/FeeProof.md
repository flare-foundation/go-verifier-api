# FeeProof

New attestation type for proper fee accounting for protocols using PMW.

## Source
XRP only (same as PMWPaymentStatus).

## Architecture
Standalone deployment, same pattern as PMWPaymentStatus (own LoadModule entry, config, service/verifier).
Reuses same data sources (Postgres source DB + MySQL C-chain index DB), can point to the same instances.
ABI structs (`IPMWFeeProofRequestBody` / `ResponseBody`) must preexist in `go-flare-common` and in the smart contracts.

## Request
- `opType`
- `senderAddress`
- `fromNonce`
- `toNonce`
- `untilTimestamp` — Flare chain block timestamp; defines the cutoff for fetching reissue events

## Response
- `actualFee` — `*big.Int` in drops, sum of executed transaction fees across all nonces in range
- `knownFee` — `*big.Int` in drops, sum of known maxFees from events across all nonces in range

## Events
Both `pay` and `reissue` emit `TeeInstructionsSent`, differentiated by operation type (`PAY` vs `REISSUE`).
- Pay instruction ID: `keccak256(abi.encode(opType, PAY, sourceId, senderAddress, nonce))`
- Reissue instruction ID: `keccak256(abi.encode(opType, REISSUE, sourceId, senderAddress, nonce, reissueNumber))`
- Multiple reissues per nonce possible (tracked by `reissueCounter`).

## Logic
1. Find all pay and reissue events for `fromNonce` to `toNonce`. Check for reissues until `untilTimestamp`.
2. For each nonce: `knownFee += pay_maxFee + sum(max(0, reissue_N_maxFee - pay_maxFee))` — add pay maxFee plus the residual of each reissue (clamped to 0 if negative).
3. Check executed transactions and sum their fees as `actualFee`.

## Error handling
- Missing transactions for any nonce in range → error (422, consistent with PMWPaymentStatus `ErrRecordNotFound`).
- Reissues after `untilTimestamp` are not the verifier's concern. Caller must first use PMWPaymentStatus to confirm `toNonce` is complete, then request FeeProof.
- Follow PMWPaymentStatus pattern: 422 for missing data, 503 for DB failures, 500 for data corruption.

## Nonce range cap
- Cap `toNonce - fromNonce` to prevent heavy queries (e.g. max 100).
- Enforce at handler level (return 400 if exceeded).
- Hardcoded constant (convention in the codebase). Actual value TBD — benchmark once queries exist.

---

## Open questions
- **Nonce range cap value** — suggested ~100, needs benchmarking.

## Implementation notes

### DB query strategy
- Current `FetchInstructionLog` queries by `topic0 + topic1 + topic2` (exact instructionID match).
- **Pay events**: instruction IDs are deterministic (`keccak(opType, PAY, sourceId, senderAddress, nonce)`). Compute all IDs for the nonce range and batch fetch in one query (up to ~100 IDs).
- **Reissue events**: instruction IDs include `reissueNumber` which is unknown upfront. Query iteratively per nonce (reissueNumber 0, 1, 2... until not found). Reissues are rare, so most nonces won't need this.
- Needs a new batch repo method (existing `FetchInstructionLog` handles one instructionID at a time).
