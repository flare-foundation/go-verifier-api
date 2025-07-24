# Go Verifier Api

- [Attestation type specification](https://docs.google.com/document/d/1i9GccSjl3ixHkShA_rnkRkchcc0D8SChM2ormumimVo/edit?tab=t.0#heading=h.p2pheiao3ip0)
- [Huma framework website](https://huma.rocks)


## How to run
1. Fill in the `.env` file

    Open `.env` and set the following values according to the attestation type you want to run:

    ```env
    VERIFIER_TYPE=TeeAvailabilityCheck       # or PMWPaymentStatus
    SOURCE_ID=tee                            # Up to 32 characters, e.g., 'xrp' or 'tee'
    PORT=3120
    ```
    - For `TeeAvailabilityCheck`, set:
    ```env
    RELAY_CONTRACT_ADDRESS=0x...
    TEE_REGISTRY_CONTRACT_ADDRESS=0x...
    RPC_URL=https://...
    ```

    - For `PMWPaymentStatus`, set:

        You will also need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

    ```env
    CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
    DATABASE_URL=postgres://user:pass@host:port/db
    ```

    ---
    To run it on Coston, for now you can copy `.env.coston` to `.env`. Please note that after the `TeeRegistryContract` is deployed, its address must be added to the `.env` file.
    ---

2. Install dependencies
    ```bash
    go mod tidy
    ```

    If there are new commits to the `tee` branch of the project `go-flare-common` project (https://github.com/flare-foundation/go-flare-common/commits/tee), you can fetch the updates using `go get https://github.com/flare-foundation/go-flare-common@<commitHash>`

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
// TODO



## TeeAvailabilityCheck TODO list
- [ ] poller.go: After TeeRegistry is added to Coston, check needs to be made to ensure, the contract is actually called.
- [ ] poller.go: If getActiveTees call on contract fails, what to do? Now it just logs it.
- [ ] poller.go: We should distinguish between invalid validation and other errors while queryTeeInfoAndValidate? Now, we don't. If error during or invalid validation -> the sample is considered invalid. This could probably lead to false proof that tee is down.
- [ ] poller.go: Split in go routines inside SampleAllTees
- [ ] pki_token.go: claims.HWModel -> what is platform enum or bytes?
- [ ] pki_token.go: If fn TeeInfoHash will be in common package, fetch from there
- [ ] verifier.go: Needs to be properly defined if response.Platform != "google" (still debating what response.Platform  is)
- [ ] verifier.go: When contracts are updated with extension regOperationConst will change to "F_REG"
- [ ] type/tee_availability_check.go: If types (TeeInfoResponse, ActionResponse) copied from tee-node will be moved to common pkg -> fetch from there

## PMWPaymentStatus TODO list
 (after TeeAvailabilityCheck will be done)
- [ ] implement http handlers
- [ ] check GetTransactionStatus if still suffices
- [ ] check GetReceivedAmount if deleted node does not spend money
- [ ] check parseRawTransactionData - should we validate required fields before further processing