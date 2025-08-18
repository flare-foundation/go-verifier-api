# Go Verifier Api

- [Attestation type specification](https://docs.google.com/document/d/1i9GccSjl3ixHkShA_rnkRkchcc0D8SChM2ormumimVo/edit?tab=t.0#heading=h.p2pheiao3ip0)
- [Huma framework website](https://huma.rocks)

## How to run `TeeAvailabilityCheck` verifier
1. Fill in the `.env` file

    Open `.env` and set the following values according to the attestation type and source you want to run:

    ```env
    VERIFIER_TYPE=TeeAvailabilityCheck       # or PMWPaymentStatus
    SOURCE_ID=tee                            # Up to 32 characters, e.g., 'xrp' or 'tee'
    PORT=3120
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
Needed for Coston deploy:
- [ ] *poller.go*: After `TeeRegistry` contract is deployed to Coston, add its address to `.env`.

<br><br>
TODO:
- [ ] ⚠️ *poller.go*: Should we distinguish between invalid validation and other errors while `queryTeeInfoAndValidate`? Now we don't. Could this lead to false accusation that tee is down? Should we distinguish between errors due to verifier and other errors?
- [ ] How to handle errors like: cannot retrieve block, cannot retrieve signingPolicy, cannot retrieve getActiveTees etc. Currently they are handled as external service problem.
- [ ] *verifier.go*: Needs to be properly defined if response.Platform != "google" (missing Platform in TeeInfoResponse)
- [ ] other TODOs inside the code


## Server TODO list
- [ ] Add security headers (something like [`helmet`](https://www.npmjs.com/package/helmet/v/6.1.2) does in `ts`). Possible candidates: `github.com/rs/cors v1.11.1` and `github.com/unrolled/secure v1.17.0`.

<br>

---
---
⚠️ `PMWPaymentStatus` is work in progress
## How to run `PMWPaymentStatus` verifier

    - For `PMWPaymentStatus`, set:

        You will also need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

    ```env
    VERIFIER_TYPE=PMWPaymentStatus       # or PMWPaymentStatus
    SOURCE_ID=xrp                            # Up to 32 characters, e.g., 'xrp' or 'tee'
    PORT=3120
    CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
    DATABASE_URL=postgres://user:pass@host:port/db
    ```

## PMWPaymentStatus TODO list
 (after TeeAvailabilityCheck will be done)
- [ ] implement http handlers
- [ ] check GetTransactionStatus if still suffices
- [ ] check GetReceivedAmount if deleted node does not spend money
- [ ] check parseRawTransactionData - should we validate required fields before further processing
