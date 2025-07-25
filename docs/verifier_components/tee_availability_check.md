# `TeeAvailabilityCheck` Main Verifier Components


## `TeeAvailabilityCheck` Verifier
The verifier ([here](https://gitlab.com/flarenetwork/tee/go-verifier-api/-/blob/main/internal/attestation/tee_availability_check/verifier/verifier.go)) handles the code logic for determining the availability of TEEs.

Key variables, constants, and details:
- The `challengeInstructionId` construction is [here](https://gitlab.com/flarenetwork/tee/go-verifier-api/-/blob/main/internal/attestation/tee_availability_check/verifier/verifier.go#L192); it uses hardcoded constants `regOperationConst = "F_REG"` and `teeAttestationConst = "TEE_ATTESTATION"`.
- The fetch timeout for the proxy's `/action/result/<challengeInstructionId>` route is `fetchTimeout = 5 * time.Second`.
- It returns HTTP 503 when verifier is indeterminate about TEE status (i.e., no result at `/action/result/<challengeInstructionId>` and all valid samples from the poller in the last 5 minutes). This indicates the request can be retried after a short delay.
- Attestation validation (JWT token signature and claims validation) happens in [here](https://gitlab.com/flarenetwork/tee/go-verifier-api/-/blob/main/internal/attestation/tee_availability_check/verifier/pki_token.go).

## Poller
The poller ([here](https://gitlab.com/flarenetwork/tee/go-verifier-api/-/blob/main/internal/attestation/tee_availability_check/poller/poller.go)) implements a periodic polling system that samples the validity of active TEEs.

Key variables, constants, and details:
- It samples at rate of `SampleInterval = 1 * time.Minute`.
- It accepts a challenge (i.e., blockHash) as "fresh" if `latestBlock.Time()-challengeBlock.Time() <= blockFreshnessInSeconds`, where `blockFreshnessInSeconds = 150s`.
- The fetch timeout for the proxy's `/info` route is `fetchTimeout = 5 * time.Second`.
- It uses a worker pool to concurrently query all active TEEs, validate their responses via verifier and maintains a rolling history of validation results for each TEE.