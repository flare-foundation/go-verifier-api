<p align="left">
  <a href="https://flare.network/" target="blank"><img src="https://content.flare.network/Flare-2.svg" width="410" height="106" alt="Flare Logo" /></a>
</p>

# Go Verifier API


## Prerequisites to Run Verifier API
Each attestation type requires certain environment variables to be set. The following are common variables needed for all attestation types:
 ```env
PORT=<port_number>
API_KEYS=<comma_separated_strings>
```

> **NOTE**: The `<port_number>` value must be consistent with the `PORT` environment variable throughout the configuration.

### `TeeAvailabilityCheck` Attestation Type
Environment variables:
 ```env
VERIFIER_TYPE=TeeAvailabilityCheck
SOURCE_ID=TEE
RELAY_CONTRACT_ADDRESS=0x...
TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS=0x...
RPC_URL=https://<flare>

# Test/E2E-only flags (optional, default to false):
ALLOW_TEE_DEBUG=false
DISABLE_ATTESTATION_CHECK_E2E=false
ALLOW_PRIVATE_NETWORKS=false
```

> **NOTE**: `ALLOW_TEE_DEBUG`, `DISABLE_ATTESTATION_CHECK_E2E`, and `ALLOW_PRIVATE_NETWORKS` are test/E2E-only flags. In production, you should leave them unset (they default to false). `ALLOW_PRIVATE_NETWORKS` permits private/loopback IPs (e.g. Docker bridge `172.17.0.1`) while still blocking dangerous IPs (link-local/metadata, multicast, Teredo, 6to4) and preserving DNS pinning.

> **WARNING: MagicPass bypass** — TEE nodes running in non-production mode (`settings.Mode != 0`) return `"magic_pass"` instead of a real attestation token. The verifier unconditionally accepts this token and skips ALL attestation validation (PKI, claims, CRL). This exists to support hackathon and development environments where real Google Confidential Space attestation is unavailable. **Do NOT rely on this in production** — any TEE returning this string will be trusted without verification.

The `TeeAvailabilityCheck` attestation type also uses Google Confidential Space Root Certificate, which is stored locally in the folder _internal/config/assets_. Read more about it [here](./internal/config/assets/README.md).

### `PMWMultisigAccountConfigured` Attestation Type
Environment variables:
```
VERIFIER_TYPE=PMWMultisigAccountConfigured
SOURCE_ID=testXRP
RPC_URL=https://<xrpl>
```

### `PMWPaymentStatus` Attestation Type
You will need to run following indexers:
(TODO replace repos links with publicly available links.)
- [xrp-indexer](https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads)
- [c-chain indexer](https://github.com/flare-foundation/flare-system-c-chain-indexer) 

Environment variables:
```env
VERIFIER_TYPE=PMWPaymentStatus
SOURCE_ID=testXRP
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
SOURCE_DATABASE_URL=postgres://user:pass@host:port/db
```

### `PMWFeeProof` Attestation Type
Requires the same indexers as `PMWPaymentStatus`.

Environment variables:
```env
VERIFIER_TYPE=PMWFeeProof
SOURCE_ID=testXRP
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
SOURCE_DATABASE_URL=postgres://user:pass@host:port/db
```

## How to Set Up and Run Verifier
1. Fill in the `.env` file or set environment variables according to the attestation type.

2. Install dependencies:

    ```bash
    go mod tidy
    ```
    To update [`go-flare-common`](https://github.com/flare-foundation/go-flare-common/commits/tee) to the latest commit on `tee` branch, run `go get github.com/flare-foundation/go-flare-common@<commitHash>`

3. Run the project:
    ```bash
    go run ./cmd/main.go
    ```

4. Access Swagger UI:
    ```
    http://localhost:<port_number>/api-doc
    ```
    Replace `<port_number>` with the value set in your `PORT` environment variable.

## API Reference
<b>Base path for all verifier endpoints</b>:
```
/verifier/<sourceName>/<attestationType>/
```
- `<sourceName>` must be lowercase.
- `<attestationType>` is the type of attestation (e.g., TeeAvailabilityCheck, PMWPaymentStatus, PMWMultisigAccountConfigured).

See [API reference](docs/api.md) for endpoint definitions and examples.

## TEE Poller
The `TeeAvailabilityCheck` attestation type initiates a process called [`tee_poller`](internal/attestation/tee_availability_check/tee_poller/tee_poller.go). The purpose of the `tee_poller` is to continuously ping all available TEEs (retrieved from the `TeeMachineRegistry` smart contract), verify the freshness of the challenge and the correctness of the attestation, and detect whether any TEEs are no longer available, which enables the system to provide a proof that a TEE machine is DOWN.

Samples retrieved by the poller can be VALID, INVALID or INDETERMINATE (the latter case occurs when the check fails due to verifier fault, e.g. being unable to connect to RPC).

Samples are stored in memory. The number of samples is defined by the constant `SamplesToConsider`, which is closely related to the constant `SampleInterval`, determining the polling interval. See [verifier file](internal/attestation/tee_availability_check/verifier/verifier.go) for reference.

## Attestation Request Submission
The process of submitting an attestation requests is as follows:

Attestation requests are triggered via TEE smart contracts. The TEE relay client, which acts as a connector between contracts on Flare's C-chain and TEE clients, listens to `TeeInstructionsSent` events with an `instructionId` that correspond to an attestation request (`FDC2_OP_TYPE` (`"F_FDC2"`) and `PROVE` (`"PROVE"`)). Each attestation request is then placed into a queue and gradually promoted to the designated verifier server. It is advised that each TEE relay client runs its own verifier server.

### Rate Limit
The blockchain itself limits how many attestation requests can be emitted per block, while the queue system enforces a controlled consumption rate for verifier servers. It is also expected that the person deploying the verifier server implements additional rate limiting at other levels.

### Security Headers
For internal-only APIs, we use a minimal set of headers:
- FrameDeny – prevent clickjacking
- ContentTypeNosniff – prevent MIME sniffing

Other headers (CORS, SSL redirect, STS, cross-origin policies) are not needed because these services are only accessed internally by trusted services, not browsers or public clients.

Minimal headers keep internal communication safe without unnecessary overhead.

## Running Tests
1. Running all tests with coverage
```bash
sh gencover.sh
```
The script is located in [gencover.sh](./gencover.sh).
- Docker services defined in [internal/tests/docker/docker-compose.yaml](./internal/tests/docker/docker-compose.yaml) will **automatically start**.
- All tests (unit + integration) will run.
- Docker services will **automatically shut down** after all tests complete.
This is the simplest way to run everything without worrying about Docker manually.

2. Running specific tests manually
- The majority of tests are **self-contained**:
    - Do **not require Docker** and can be run directly:
        ```bash
        go test -v <path_to_test>
        ```
- A few tests, related to **PMWPaymentStatus attestation type** are **Docker dependant tests** (e.g., tests that access the indexer databases).
    > Note: These tests include a comment in the test file marking them as Docker-dependent.
    - Start Docker manually:
        ```bash
        docker compose -f internal/tests/docker/docker-compose.yaml up -d
        ```
    - Run the test:
        ```bash
        go test -v <path_to_test>
        ```
    - Stop Docker after finishing:
        ```bash
        docker compose -f internal/tests/docker/docker-compose.yaml down
        ```

## TODO (to-think-about) list
- Other `TODO`s inside the code and README.
- How often should we query GetAllActiveTeeMachines? At the moment, each poll also retrieves GetAllActiveTeeMachines.
- TEEAvailabilityCheck currently supports only "google". When support for other platforms is added, TeeInfo.Platform needs to be added in order to know, how to decode the data.
- PMWFeeProof: Benchmark and adjust `MaxNonceRange` (currently 100).
- PMWFeeProof: Should the verifier validate that the requested nonce range falls within the XRP indexer data retention window (~2 weeks), or just let it fail with 422 if data is missing?
- PMWFeeProof: Confirm with FAsset team that the `estimatedFee` formula (`pay_maxFee + sum(max(0, reissue_maxFee - pay_maxFee))`) is suitable for their fee reconciliation use case.

### Monitoring
- When the `TeeAvailabilityCheck` verifier is running, poller samples should be monitored via the `/poller/tees` route to ensure that timestamps are recent enough, allowing early detection of poller failures.

## Technical Specification
See [docs/SPEC.md](docs/SPEC.md) for the full technical specification covering architecture, verification flows, error model, and configuration.