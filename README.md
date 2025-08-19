# Go Verifier Api

- [Attestation types specification](https://docs.google.com/document/d/1i9GccSjl3ixHkShA_rnkRkchcc0D8SChM2ormumimVo/edit?tab=t.0#heading=h.p2pheiao3ip0)
- [Huma framework website](https://huma.rocks)

## How to run `TeeAvailabilityCheck` verifier
1. Fill in the `.env` file

    Open `.env` and set the following values according to the attestation type and source you want to run:

    ```env
    VERIFIER_TYPE=TeeAvailabilityCheck
    SOURCE_ID=tee
    RELAY_CONTRACT_ADDRESS=0x...
    TEE_REGISTRY_CONTRACT_ADDRESS=0x...
    RPC_URL=https://...
    ```
    ---
    To run it on Coston, for now you can copy `.env.coston` to `.env`. Please note that after the `TeeRegistryContract` is deployed, its address must be added to the `.env` file.
    ---

2. Install dependencies
    ```bash
    go mod tidy
    ```

    If there are new commits to the `tee` branch of the project `go-flare-common` project (https://github.com/flare-foundation/go-flare-common/commits/tee), you can fetch the updates using `go get github.com/flare-foundation/go-flare-common@<commitHash>`

3. Run the project
    ```bash
    go run ./cmd/main.go
    ```

4. Access Swagger UI
    ```
    http://localhost:3120/api-doc
    ```

## API Reference
See [API reference](docs/api.md) for endpoint definitions and examples.

## File structure
See [File structure](docs/overview.md) for a detailed explanation of the directory layout.

## TeeAvailabilityCheck TODO list
- [ ] ⚠️ *poller.go*: Should we distinguish between invalid validation and other errors while `queryTeeInfoAndValidate`? Now we don't. Could this lead to false accusation that tee is down? Should we distinguish between errors due to verifier and other errors?
- [ ] How to handle errors like: cannot retrieve block, cannot retrieve signingPolicy, cannot retrieve getActiveTees etc. Currently they are handled as external service problem.
- [ ] *verifier.go*: Needs to be properly defined if response.Platform != "google" (missing Platform in TeeInfoResponse)
- [ ] Other `TODO`s inside the code.


## How to run `PMWMultisigAccountConfigured` verifier
⚠️ `PMWPaymentStatus` is work in progress
```
VERIFIER_TYPE=PMWMultisigAccountConfigured
SOURCE_ID=testxrp
RPC_URL=https://s.altnet.rippletest.net:51234/
```

## How to run `PMWPaymentStatus` verifier
⚠️ `PMWPaymentStatus` is work in progress


You will also need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

```env
VERIFIER_TYPE=PMWPaymentStatus
SOURCE_ID=testxrp
CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
DATABASE_URL=postgres://user:pass@host:port/db
```

## PMWPaymentStatus TODO list
- [ ] implement http handlers
- [ ] check GetTransactionStatus if still suffices
- [ ] check GetReceivedAmount if deleted node does not spend money
- [ ] check parseRawTransactionData - should we validate required fields before further processing
