# Verifier Api

This is the start repo to introduce FTDC attestation types.

Docs: https://docs.google.com/document/d/1i9GccSjl3ixHkShA_rnkRkchcc0D8SChM2ormumimVo/edit?tab=t.0#heading=h.p2pheiao3ip0

## On the top level:
- [x] Add in readme "How to run" (but more like write about local requirements)
- [x] Missing "general" API framework (like it is in current https://gitlab.com/flarenetwork/fdc/verifier-indexer-api)
- [ ]
- [ ]
- [ ]

## Tee availability check

- [ ] /info response from TEE proxy needs to be defined
- [ ] data to include in eat_nonce needs to be defined
- [x] should new status MISMATCH be added
- [x] challenge between tee and proxy need to be defined for /info
- [x] missing logic to prove /info has fresh data

## PMWPaymentStatus

- [ ] check GetTransactionStatus if still suffices
- [ ] check GetReceivedAmount if deleted node does not spend money
- [ ] check parseRawTransactionData - should we validate required fields before further processing

## How to run
1. Fill in the `.env` file

    Open `.env` and set the following values according to the attestation type you want to run:

    ```env
    VERIFIER_TYPE=TeeAvailabilityCheck       # or PMWPaymentStatus
    SOURCE_ID=tee                            # Up to 32 characters, e.g., 'xrp' or 'tee'
    PORT=3120
    ```
    For `TeeAvailabilityCheck`, set:
    ```env
    RELAY_CONTRACT_ADDRESS=0x...
    TEE_REGISTRY_CONTRACT_ADDRESS=0x...
    RPC_URL=https://...
    ```

    For `PMWPaymentStatus`, set:
    ```env
    CCHAIN_DATABASE_URL=user:pass@tcp(host:port)/db?parseTime=true
    DATABASE_URL=postgres://user:pass@host:port/db
    ```
    You will also need to run https://gitlab.com/flarenetwork/fdc/verifier-xrp-indexer/-/tree/add-new-fields?ref_type=heads and https://gitlab.com/flarenetwork/FSP/flare-system-c-chain-indexer.

2. Install dependencies
    ```bash
    go mod tidy
    ```

3. Run the project
    ```bash
    go run ./cmd/main.go
    ```

4. Access Swagger UI
    ```
    localhost:3120/swagger/index.html
    ```

<!-- ## Tools (experimental)

https://www.alexedwards.net/blog/how-to-manage-tool-dependencies-in-go-1.24-plus
```
go mod vendor
go tool -n staticcheck 
```
- Upgrade the module `go get github.com/golang-jwt/jwt/v4@latest`
- Clean up unused modules `go mod tidy`
- Sync vendor dir (important!) `go mod vendor`
- Re-run audit `make audit` -->