# Go Verifier Api

- [Attestation types specification](https://docs.google.com/document/d/1i9GccSjl3ixHkShA_rnkRkchcc0D8SChM2ormumimVo/edit?tab=t.0#heading=h.p2pheiao3ip0)
- [Huma framework website](https://huma.rocks)
- [Formatters and Linters](https://gitlab.com/flarenetwork/flare-handbook/-/tree/main/resources/tech_stack/golang)

## How to run `TeeAvailabilityCheck` verifier
Environment variables:
 ```env
VERIFIER_TYPE=TeeAvailabilityCheck
SOURCE_ID=TEE
RELAY_CONTRACT_ADDRESS=0x...
TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS=0x...
RPC_URL=https://<flare>
```

## How to run `PMWMultisigAccountConfigured` verifier
Environment variables:
```
VERIFIER_TYPE=PMWMultisigAccountConfigured
SOURCE_ID=XRP
RPC_URL=https://<xrpl>
```

## How to run `PMWPaymentStatus` verifier

You will need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

Environment variables:
```env
VERIFIER_TYPE=PMWPaymentStatus
SOURCE_ID=XRP
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
DATABASE_URL=postgres://user:pass@host:port/db
```

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
- [ ] *verifier.go*: Needs to be properly defined if response.Platform != "google" (missing Platform in TeeInfoResponse).
- [ ] Other `TODO`s inside the code.
- [ ] Check which types and functions can be fetched from other packages (go-flare-common, tee-node).
- [ ] `verify` route: support json friendly inputs or have direct types from other packages? - We will define api friendly types between relay-client and go-verifier-api.
- [ ] PMWPaymentStatus: is there a way to avoid using `string` for `RevertReason`.
- [ ] Add poller API endpoint in readme.
- [ ] Check all ctx's are sensible -> retry with backoff (wait 5s).