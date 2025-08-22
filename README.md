# Go Verifier Api

- [Attestation types specification](https://docs.google.com/document/d/1i9GccSjl3ixHkShA_rnkRkchcc0D8SChM2ormumimVo/edit?tab=t.0#heading=h.p2pheiao3ip0)
- [Huma framework website](https://huma.rocks)
- [Formatters and Linters](https://gitlab.com/flarenetwork/flare-handbook/-/tree/main/tech_stack/golang?ref_type=heads)

## How to run `TeeAvailabilityCheck` verifier
`.env` variables:
 ```env
VERIFIER_TYPE=TeeAvailabilityCheck
SOURCE_ID=tee
RELAY_CONTRACT_ADDRESS=0x...
TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS=0x...
RPC_URL=https://<flare>
```

## How to run `PMWMultisigAccountConfigured` verifier
`.env` variables:
```
VERIFIER_TYPE=PMWMultisigAccountConfigured
SOURCE_ID=testxrp
RPC_URL=https://<xrpl>
```

## How to run `PMWPaymentStatus` verifier

You will need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

`.env` variables:
```env
VERIFIER_TYPE=PMWPaymentStatus
SOURCE_ID=testxrp
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
DATABASE_URL=postgres://user:pass@host:port/db
RPC_URL=https://<flare>
TEE_WALLET_MANAGER_CONTRACT_ADDRESS=
TEE_WALLET_PROJECT_MANAGER_CONTRACT_ADDRESS=
```

## How to run setup verifier
1. Fill in the `.env` file according to the attestation type.

2. Install dependencies:
    ```bash
    go mod tidy
    ```

    If there are new commits to the `tee` branch of the project [`go-flare-common`](https://github.com/flare-foundation/go-flare-common/commits/tee), you can fetch the updates using `go get github.com/flare-foundation/go-flare-common@<commitHash>`

    Package [tee-node](https://gitlab.com/flarenetwork/tee/tee-node) should be pulled locally.

3. Run the project:
    ```bash
    go run ./cmd/main.go
    ```

4. Access Swagger UI:
    ```
    http://localhost:3120/api-doc
    ```

## API Reference
See [API reference](docs/api.md) for endpoint definitions and examples.

## TeeAvailabilityCheck TODO list
- [ ] ⚠️ *poller.go*: Should we distinguish between invalid validation and other errors while `queryTeeInfoAndValidate`? Now we don't. Could this lead to false accusation that tee is down? Should we distinguish between errors due to verifier and other errors?
- [ ] How to handle errors like: cannot retrieve block, cannot retrieve signingPolicy, cannot retrieve getActiveTees etc. Currently they are handled as external service problem.
- [ ] *verifier.go*: Needs to be properly defined if response.Platform != "google" (missing Platform in TeeInfoResponse)
- [ ] Other `TODO`s inside the code.


