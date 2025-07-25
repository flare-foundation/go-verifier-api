# API Reference

This API exposes a POST endpoints to verify the availability of a Trusted Execution Environment (TEE).

## 1. Main endpoint `POST /TeeAvailabilityCheck/verify`
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
  "responseBody": "0x0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
}
```

# Response statuses:
| HTTP Status Code           | Description          |
|----------------------------|----------------------|
| 200 OK                     | The request succeeded.
| 400 Bad Request            | Request body validation failed (e.g., missing or invalid fields, or conversion, encoding, or decoding errors). |
| 503 Service Unavailable    | Indeterminate status - the request can be retried.
| 500 Internal Server Error  | Any other errors, with description provided in the `detail` field.

# Data Structures

1. Full attestation request:
```json
{
  "header": TeeAvailabilityHeader,
  "requestBody": "0x..." // hex-encoded bytes
}
```

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

- `requestBody`: a hex-encoded byte string representing an ABI-encoded structure `TeeAvailabilityRequestData`

Decoded `TeeAvailabilityRequestData`
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

2. Attestation response:
```
{
  "requestBody": "0x..." // hex-encoded bytes
}
```

- `responseBody`: a hex-encoded byte string representing an ABI-encoded structure `TeeAvailabilityResponseData`
Decoded `TeeAvailabilityResponseData`
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
| status                 | number | Enum  AvailabilityCheckStatus { OK, OBSOLETE, DOWN }
| teeTimestamp           | uint64 | TEE timestamp
| codeHash               | string |	32-byte hex-encoded SHA-256 digest of the workload container image (from JWT)
| platform               | string | 32-byte hex-encoded hwmodel (from JWT)
| initialSigningPolicyId | uint32 | initial signing policy id
| lastSigningPolicyId    | uint32 |	last signing policy id
| stateHash              | string |	32-byte hex-encoded state


## 2. Helper endpoint `POST /TeeAvailabilityCheck/prepareRequestBody`
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
  "requestData": {
    "challenge": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "teeId": "0x000000000000000000000000000000000000dEaD",
    "url": "https://proxy.url/tee"
  }
}
```
### Response:
```json
{
  "requestBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"
}
```

## 3. Helper endpoint `POST /TeeAvailabilityCheck/prepareResponseBody`
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
  "responseData": {
    "status": 0,
    "teeTimestamp": 123456789,
    "codeHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "platform": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "initialSigningPolicyId": 1,
    "lastSigningPolicyId": 2,
    "stateHash": "0x0000000000000000000000000000000000000000000000000000000000000000"
  },
  "responseBody": "0x0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
}
```