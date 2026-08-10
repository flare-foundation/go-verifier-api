# Design note: one deployment per source (multiple attestation types)

**Status:** proposal / for sign-off
**Audience:** DevOps + verifier engineering
**Author:** (fill in)

## Problem

Today the verifier deploys **one process per `(attestation type, source)` pair**.
With four attestation types across their sources that is four separate
deployments:

| Deployment | `VERIFIER_TYPE` | `SOURCE_ID` |
|------------|-----------------|-------------|
| 1 | `TeeAvailabilityCheck` | `TEE` |
| 2 | `PMWPaymentStatus` | `XRP` |
| 3 | `PMWFeeProof` | `XRP` |
| 4 | `PMWMultisigAccountConfigured` | `XRP` |

DevOps asked whether we can instead run **one deployment per source** that serves
all of that source's attestation types.

## Answer

Yes, and it is a small, contained change — the architecture already supports the
hard part. It also matches how **FDC1 already runs in production** (see
Precedent).

Proposed shape: **two deployments instead of four.**

| Deployment | Source | Attestation types served |
|------------|--------|--------------------------|
| `verifier-xrp` | XRP | `PMWPaymentStatus`, `PMWFeeProof`, `PMWMultisigAccountConfigured` |
| `verifier-tee` | TEE | `TeeAvailabilityCheck` |

`TeeAvailabilityCheck` stays on its own — its source *is* `TEE`, it has no
database dependency, and it uses a different contract set and trust surface
(it validates Confidential Space attestation tokens). It is already a separate
source, so grouping does not apply to it.

## Precedent: FDC1 already does this

The FDC1 verifier (`verifier-indexer-api`, NestJS) already ships the per-source
model, and it is worth mirroring rather than inventing our own:

- **One universal image** (single `Dockerfile`, single `dist/main`) — no
  per-source build variants.
- **One env var selects the source at runtime.** `VERIFIER_TYPE=btc|xrp|eth|…`
  drives a `switch` in `src/verifierServer.ts` that picks a **per-source module**
  (`BtcVerifierServerModule`, `XRPVerifierServerModule`, …).
- **The set of attestation types per source is fixed in that module**, not
  configured. e.g. the BTC module registers AddressValidity + BalanceDecreasing
  + ConfirmedBlockHeight + Payment + ReferencedPaymentNonexistence; XRP registers
  those plus the two XRP-specific types.
- **Deployment = same image, N services**, one per source, differing only by the
  `VERIFIER_TYPE` env (see `docker/verifier-evm/compose.yml`: `verifier_eth`,
  `verifier_flr`, … all `build: ../../`).

Net: an FDC1 source deployment serves **all of that source's types, fixed in
code** — it is not a per-deployment opt-in list. That is the model DevOps is
asking for, already proven.

## Why it is close already (FDC2 / this repo)

### Routing is already namespaced by source **and** type

Every endpoint is registered under:

```
/verifier/{source}/{type}/{endpoint}
    e.g. /verifier/xrp/PMWPaymentStatus/verify
```

(`getVerifierAPIPath`, `internal/api/handler/handler_util.go`). Multiple types on
one server therefore **cannot collide** — each type gets its own route subtree.
This is stricter than FDC1 (whose routes are not source-prefixed because each
process is a single source), so we are actually better positioned.

### Registration and lifecycle are already multi-service shaped

- `RegisterVerificationHandler` (`internal/api/handler/handler.go`) is a
  per-verifier call — invoked once per type today.
- `LoadModule` (`internal/api/loader.go`) already returns `[]io.Closer`, and
  `StartServer` / `ShutdownServer` (`internal/api/server.go`) already iterate that
  slice to shut down N services cleanly.

**No changes needed to:** routing, the verifier implementations, the handler
layer, or the tee-relay-client wire contract.

## What actually needs to change

1. **Selection should be source-driven, like FDC1.** Today `LoadEnvConfig`
   (`internal/api/server.go`) reads a single `VERIFIER_TYPE` (an *attestation
   type*), and `LoadModule` is a `switch` over that one type. Mirror FDC1: let
   **`SOURCE_ID` select the deployment**, and register **all attestation types
   known for that source** via a hardcoded `source → []attestationType` map. So
   `SOURCE_ID=XRP` registers `PMWPaymentStatus` + `PMWFeeProof` +
   `PMWMultisigAccountConfigured`. This is FDC1's per-source module expressed in
   Go, and it directly answers "all of that source's types" (fixed in code, not a
   per-deploy list). `LoadModule`'s `switch` becomes a loop over the map entry.

   *Optional escape hatch:* also honour an explicit override (e.g.
   `VERIFIER_TYPES=…`) so a single type can be split back out via config without
   a code change. Default = the full per-source set.

2. **Shared config values.** Per-type config is read from shared env var *names*
   (`RPC_URL`, `CCHAIN_DATABASE_URL`, `SOURCE_DATABASE_URL`,
   `FLARE_TEE_MANAGER_CONTRACT_ADDRESS`, `TEE_PAYMENTS_CONTRACT_ADDRESS`). For a
   single source these are the **same** across its types (same source RPC, same
   C-chain DB, same source-indexer DB, same contracts), so the XRP trio shares
   config with no per-type prefixing. If a future type on the same source ever
   needs a *different* value for a shared name, introduce a per-type prefix then.

3. **Connection fan-out (minor).** Each service opens its own DB pools and RPC
   client. Two XRP types in one process means two sets of connections to the same
   DBs/RPC. Functionally fine; can be optimised to share pools later if
   connection count matters.

4. **Cosmetic:** the OpenAPI title/description and the startup log line name a
   single type (`newAPI`, `StartServer`) — generalise to list the served types.

## Relay side

Config-only, **no code change**. The relay posts each verifier request to a
per-verifier URL that already contains the type in its path. DevOps points the
three XRP verifier entries at the **same host:port** (their existing distinct
paths) instead of three separate hosts.

## Trade-offs

Consolidation costs — acceptable today, named for the record:

- **Fault isolation.** An OOM or panic in one type takes down the others on that
  source. These are stateless, read-only verifiers behind a retrying relay, with
  multiple verifiers in the FDC set, so a blip degrades gracefully rather than
  corrupting anything — but the blast radius of a bad deploy grows. (FDC1 accepts
  this same trade-off.)
- **Independent scaling / deploy cadence.** You can no longer scale or roll out
  one type without the others. The optional override in change #1 is the escape
  hatch: split a heavy type back out via config, no code change.

## When consolidation is *not* the right lever

If the motivation is only "four near-identical Kubernetes manifests are
annoying," that is better solved with a Helm/kustomize template than by
consolidating processes. Consolidation pays off when the driver is **runtime
cost** — fewer Go runtimes, fewer idle baselines, shared connection pools.
Confirm which it is before building.

## Effort

Moderate and contained (~1–2 days), no architectural surgery:

- source → types map + source-driven selection in `LoadEnvConfig` (plus optional
  override),
- refactor `LoadModule`'s `switch` into a loop over the source's types,
- update the `.env.*` templates,
- extend the loader and server integration tests (multi-type registration).

## Decision requested

1. Proceed with **FDC1-parity, source-driven** registration — `SOURCE_ID`
   selects the deployment, all of that source's types are registered — grouped as
   `verifier-xrp` (PMW trio) + `verifier-tee`?
2. Keep the **optional per-deploy override** as an escape hatch, or omit it for
   strict FDC1 parity?
3. Is the driver **runtime cost** (consolidate) or **manifest count** (template
   instead)?
