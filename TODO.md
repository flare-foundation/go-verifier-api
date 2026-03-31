# Poller Extension Filtering

## Status: Implemented

## Problem
As extensions and machines multiply (especially in development), the verifier polls all active TEE machines regardless of extension. This can overload the poller.

## Implementation
1. Call `getActiveTeeMachines(0)` — extension 0 machines are **always** polled (mandatory, guaranteed DOWN proof capability).
2. If `MAX_POLLED_TEES > 0`, call `getAllActiveTeeMachines` — from the remaining machines (not in extension 0), poll up to the cap.
3. `MAX_POLLED_TEES` env var — default 0 (extension 0 only). If set to e.g. 50 and extension 0 has 10 machines, up to 40 extra machines from other extensions are polled. Extension 0 machines are never capped.

## Contract Support
- `getActiveTeeMachines(uint256 _extensionId)` already exists in `ITeeMachineRegistry` — returns machines for a specific extension (no pagination).
- `getAllActiveTeeMachines(uint256 _start, uint256 _end)` — returns all machines with pagination (current implementation).

## Load Test Results (Poller Scaling)

With the current `defaultWorkerCount = 10` and `SampleInterval = 1 minute`:

| Scenario | Cycle Time | Status |
|---|---|---|
| 10 TEEs, all healthy | 12ms | OK |
| 50 TEEs, all healthy | 57ms | OK |
| 100 TEEs, all healthy | 117ms | OK |
| 200 TEEs, all healthy | 233ms | OK |
| 500 TEEs, all healthy | 593ms | OK |
| 100 TEEs, 25 slow (2s) | 6s | OK |
| **500 TEEs, 200 slow (4s)** | **80s** | **Exceeds 1-minute interval** |

The last scenario confirms that a large number of slow/unhealthy TEEs can cause the poll cycle to exceed the polling interval. This supports the need for extension filtering and a cap on polled machines.

## Context
- Machines outside extension 0 that go down can be paused by a separate bot after attestation validity expires — no DOWN proof required.
- The verifier does not guarantee DOWN proofs for machines outside extension 0.
- A separate bot handles expiry-based pausing (out of scope for the verifier).
