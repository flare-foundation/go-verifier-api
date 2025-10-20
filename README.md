<p align="left">
  <a href="https://flare.network/" target="blank"><img src="https://content.flare.network/Flare-2.svg" width="410" height="106" alt="Flare Logo" /></a>
</p>

# Go Verifier API


## How to run  Verifier API
Check all enviroment variables in [.env.example](./.env.example)

### `TeeAvailabilityCheck` attestation type
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
```

> **NOTE**: `ALLOW_TEE_DEBUG` and `DISABLE_ATTESTATION_CHECK_E2E` are test/E2E-only flags. In production, you can leave them unset (they default to false).

The `TeeAvailabilityCheck` attestation type also uses Google Confidential Space Root Certificate, which is stored locally in the folder _internal/attestation/tee_availability_check/config/assets_. Read more about it [here](./internal/attestation/tee_availability_check/config/assets/README.md).

### `PMWMultisigAccountConfigured` attestation type
Environment variables:
```
VERIFIER_TYPE=PMWMultisigAccountConfigured
SOURCE_ID=testXRP
RPC_URL=https://<xrpl>
```

### `PMWPaymentStatus` attestation type
You will need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

Environment variables:
```env
VERIFIER_TYPE=PMWPaymentStatus
SOURCE_ID=testXRP
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
SOURCE_DATABASE_URL=postgres://user:pass@host:port/db
```

## How to setup verifier
1. Fill in the `.env` file or use environment variables according to the attestation type.

2. Install dependencies:

    Ensure the [tee-node](https://gitlab.com/flarenetwork/tee/tee-node) package is cloned locally.

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
    http://localhost:3120/api-doc
    ```

## API Reference
<b>Base path for all verifier endpoints</b>:
```
/verifier/<sourceName>/<attestationType>/
```
- `<sourceName>` must be lowercase.
- `<attestationType>` is the type of attestation (e.g., TeeAvailabilityCheck, PMWPaymentStatus, PMWMultisigAccountConfigured).

See [API reference](docs/api.md) for endpoint definitions and examples.

## TEE poller
The `TeeAvailabilityCheck` attestation type initiates a process called [`tee_poller`](internal/attestation/tee_availability_check/tee_poller/tee_poller.go). The purpose of the `tee_poller` is to continuously ping all available TEEs (retrieved from the `TeeMachineRegistry` smart contract), verify the freshness of the challenge and the correctness of the attestation, and detect whether any TEEs are no longer available, which enables to provide a proof that a TEE machine is DOWN.

Samples retrieved by the poller can be VALID, INVALID or INDETERMINATE (the latter case occurs when the check fails due to verifier fault, e.g. being unable to connect to RPC).

Samples are stored in memory. The number of samples is defined by the constant `SamplesToConsider`, which is closely related to the constant `SampleInterval`, determining the polling interval. See [verifier file](internal/attestation/tee_availability_check/verifier/verifier.go) for reference.

## Attestation request submission
The process of submitting an attestation requests is as follows:

Attestation requests are triggered via TEE smart contracts. The TEE relay client, which acts as a connector between contracts on Flare's C-chain and TEE clients, listens to `TeeInstructionsSent` events with an `instructionId` that correspond to an attestation request (`FTDC_OP_TYPE` (`"F_FTDC"`) and `PROVE` (`"PROVE"`)). Each attestation request is than placed into a queue and gradually promoted to the designated verifier server. It is advised that each TEE relay client runs its own verifier server.

### Rate limit
The blockchain itself limits how many attestation requests can be emitted per block, while the queue system enforces a controlled consumption rate for verifier servers. It is also expected that the person deploying the verifier server implements additional rate limiting at other levels.

### Security Headers

For internal-only APIs, we use a minimal set of headers:
- FrameDeny – prevent clickjacking
- ContentTypeNosniff – prevent MIME sniffing

Other headers (CORS, SSL redirect, STS, cross-origin policies) are not needed because these services are only accessed internally by trusted services, not browsers or public clients.

Minimal headers keep internal communication safe without unnecessary overhead.

## TODO list
- [ ] Other `TODO`s inside the code.
- [ ] How often should we query GetAllActiveTeeMachines? At the moment, each poll also retrieves GetAllActiveTeeMachines.
