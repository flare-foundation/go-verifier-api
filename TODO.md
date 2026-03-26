# Poller Extension Filtering

## Problem
As extensions and machines multiply (especially in development), the verifier polls all active TEE machines regardless of extension. This can overload the poller.

## Agreed Approach
1. Call `getActiveTeeMachines(0)` — extension 0 machines are **always** polled (mandatory, guaranteed DOWN proof capability).
2. Call `getAllActiveTeeMachines` — from the remaining machines (not in extension 0), poll up to a configurable cap.
3. Add `MAX_POLLED_MACHINES` env var — if extension 0 has 10 machines and cap is 50, up to 40 extra machines from other extensions are polled. Extension 0 machines are never capped.

## Contract Support
- `getActiveTeeMachines(uint256 _extensionId)` already exists in `ITeeMachineRegistry` — returns machines for a specific extension (no pagination).
- `getAllActiveTeeMachines(uint256 _start, uint256 _end)` — returns all machines with pagination (current implementation).

## Context
- Machines outside extension 0 that go down can be paused by a separate bot after attestation validity expires — no DOWN proof required.
- The verifier does not guarantee DOWN proofs for machines outside extension 0.
- A separate bot handles expiry-based pausing (out of scope for the verifier).
