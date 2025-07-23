# API Reference

This API exposes a POST endpoints to verify the availability of a Trusted Execution Environment (TEE).

## 1. `POST /TeeAvailabilityCheck/prepareRequestBody`
Returns ABI-encoded `TeeAttestationAvailabilityRequest` request data. This helper endpoint generates the ABI-encoded `requestBody`.

- Note: Currently, this endpoint only performs encoding. Verification functionality will be added later.

### Request:
```json
{
  "header": {
    "attestationType": "0x546565417661696c6162696c697479436865636b000000000000000000000000",
    "cosigners": [],
    "cosignersThreshold": 0,
    "sourceId": "0x7465650000000000000000000000000000000000000000000000000000000000",
    "thresholdBIPS": 0
  },
  "requestBody": {
    "challenge": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "teeId": "0x000000000000000000000000000000000000dEaD",
    "url": "https://proxy.url/tee"
  }
}
```
### Response:
```json
{
  "encodedRequestBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"
}
```
### Errors:
| HTTP Status Code           | Description          |
|----------------------------|----------------------|
| 400 Bad Request            | Request body validation failed (e.g., missing or invalid fields, conversion, encoding, or decoding failures) |
| 500 Internal Server Error  | Any other errors, with description provided in the `detail` field 


## 2. `POST /TeeAvailabilityCheck/prepareResponseBody`
Verify the encoded request body and returns both the decoded `TeeAttestationAvailabilityResponse` and its ABI-encoded form.


### Request:
```json
{
  "header": {
    "attestationType": "0x546565417661696c6162696c697479436865636b000000000000000000000000",
    "cosigners": [],
    "cosignersThreshold": 0,
    "sourceId": "0x7465650000000000000000000000000000000000000000000000000000000000",
    "thresholdBIPS": 0
  },
  "requestBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"
}
```
### Response:
```json
{
  "responseBody": {
    "status": 0,
    "teeTimestamp": 123456789,
    "codeHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "platform": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "initialSigningPolicyId": 1,
    "lastSigningPolicyId": 2,
    "stateHash": "0x0000000000000000000000000000000000000000000000000000000000000000"
  },
  "encodedResponseBody": "0x0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
}
```
### Errors:
| HTTP Status Code           | Description          |
|----------------------------|----------------------|
| 400 Bad Request            | Request body validation failed (e.g., missing or invalid fields, conversion, encoding, or decoding failures) |
| 500 Internal Server Error  | Any other errors, with description provided in the `detail` field 

## 3. `POST /TeeAvailabilityCheck/verify`
Verify the encoded request body and returns ABI-encoded `TeeAttestationAvailabilityResponse`.


### Request:
```json
{
  "header": {
    "attestationType": "0x546565417661696c6162696c697479436865636b000000000000000000000000",
    "cosigners": [],
    "cosignersThreshold": 0,
    "sourceId": "0x7465650000000000000000000000000000000000000000000000000000000000",
    "thresholdBIPS": 0
  },
  "requestBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"
}
```
### Response:
```json
{
  "encodedResponseBody": "0x0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
}
```
### Errors:
| HTTP Status Code           | Description          |
|----------------------------|----------------------|
| 400 Bad Request            | Request body validation failed (e.g., missing or invalid fields, conversion, encoding, or decoding failures) |
| 500 Internal Server Error  | Any other errors, with description provided in the `detail` field 


# Data Structures

- `TeeAvailabilityHeader` has shared metadata used in requests.
```json
{
  "attestationType": "0x546565417661696c6162696c697479436865636b000000000000000000000000",
  "sourceId": "0x7465650000000000000000000000000000000000000000000000000000000000",
  "thresholdBIPS": 0,
  "cosigners": [],
  "cosignersThreshold": 0
}
```

| Field              | Type     | Description            |
|--------------------|----------|-----------------------|
| attestationType    | string   | 32-byte hex-encoded identifier of the attestation type
| sourceId           | string   | 32-byte hex-encoded source identifier
| thresholdBIPS      | uint16   | Not relevant for verifier
| cosigners          | []string | Not relevant for verifier
| cosignersThreshold | uint64   | Not relevant for verifier


- `TeeAvailabilityRequestBody`
```json
{
  "teeId": "0x000000000000000000000000000000000000dEaD",
  "url": "https://proxy.url/tee",
  "challenge": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
}
```
| Field    | Type   | Description
|----------|--------|----------------------|
| teeId    | string | Ethereum address of the TEE
| url      | string | TEE proxy endpoint URL
| challenge| string | 32-byte hex-encoded challenge hash

- `TeeAvailabilityResponseBody`
```json
{
  "status": 0,
  "teeTimestamp": 123456789,
  "codeHash": "0x...",
  "platform": "0x...",
  "initialSigningPolicyId": 1,
  "lastSigningPolicyId": 2,
  "stateHash": "0x..."
}
```
| Field                  | Type   | Description
|------------------------|--------|----------------------|
| status                 | number | Ethereum address of the TEE
| teeTimestamp           | uint64 | TEE timestamp
| codeHash               | string |	32-byte hex-encoded SHA-256 digest of the workload container image
| platform               | string | //TBD - TODO
| initialSigningPolicyId | uint32 | initial signing policy id
| lastSigningPolicyId    | uint32 |	last signing policy id
| stateHash              | string |	32-byte hex-encoded state

- `EncodedRequestBody`
Used for ABI-encoded request bodies:
```json
{
  "encodedRequestBody": "0x0000abcd...",
}
```

- `EncodedResponseBody`
Used for ABI-encoded response bodies:
```json
{
  "encodedResponseBody": "0x0000abcd...",
}
```