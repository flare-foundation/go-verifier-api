# Go Verifier API

## How to run `TeeAvailabilityCheck` verifier
Environment variables:
 ```env
VERIFIER_TYPE=TeeAvailabilityCheck
SOURCE_ID=TEE
RELAY_CONTRACT_ADDRESS=0x...
TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS=0x...
RPC_URL=https://<flare>
ALLOW_TEE_DEBUG=false
DISABLE_ATTESTATION_CHECK_E2E=false
```

## How to run `PMWMultisigAccountConfigured` verifier
Environment variables:
```
VERIFIER_TYPE=PMWMultisigAccountConfigured
SOURCE_ID=testXRP
RPC_URL=https://<xrpl>
```

## How to run `PMWPaymentStatus` verifier
You will need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

Environment variables:
```env
VERIFIER_TYPE=PMWPaymentStatus
SOURCE_ID=testXRP
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
SOURCE_DATABASE_URL=postgres://user:pass@host:port/db
```

Check all enviroment variables in [.env.example](./.env.example)

## How to run setup verifier
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

## TODO list
- [ ] Other `TODO`s inside the code.
- [ ] PMWPaymentStatus: is there a way to avoid using `string` for `RevertReason`.
- [ ] Improve healthy endpoint (all needed services are running).
- [ ] Jakob: Place DisableAttestationCheckE2E properly in order to use TEEAvailabilityCheck verifier directly in e2e test.