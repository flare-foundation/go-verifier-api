# Verifier Api

This is the start repo to introduce FDTC attestation types.


Docs: https://docs.google.com/document/d/1i9GccSjl3ixHkShA_rnkRkchcc0D8SChM2ormumimVo/edit?tab=t.0#heading=h.p2pheiao3ip0


## On the top level:
- [ ] Missing "general" API framework (like it is in current https://gitlab.com/flarenetwork/fdc/verifier-indexer-api)
- [ ] 
- [ ] 
- [ ] 
- [ ] 



## Tee availability check

- [ ] /info response from TEE proxy needs to be defined
- [ ] data to include in eat_nonce needs to be defined
- [ ] should new status MISMATCH be added
- [ ] challenge between tee and proxy need to be defined for /info
- [ ] missing logic to prove /info has fresh data

## PMWPaymentStatus

- [ ] check GetTransactionStatus if still suffices
- [ ] check GetReceivedAmount if deleted node does not spend money
- [ ] check parseRawTransactionData - should we validate required fields before further processing
- [ ] 
- [ ] 