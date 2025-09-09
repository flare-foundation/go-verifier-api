# API Reference

This API exposes a POST endpoints to verify different attestation types.

<b>Base path for all verifier endpoints</b>:
```
/verifier/<sourceName>/<attestationType>/
```
- `<sourceName>` must be lowercase.
- `<attestationType>` is the type of attestation (e.g., TeeAvailabilityCheck, PMWPaymentStatus, PMWMultisigAccountConfigured).

## 1. Main endpoint `POST /verifier/<sourceName>/<attestationType>/verify`
Verify the encoded request body and returns ABI-encoded response.
### Request:
```json
{
  "attestationType": "0x546...",
  "sourceID": "0x746...",
  "requestBody": "0x0ab..."
}
```
### Response:
```json
{
  "responseBody": "0x2de..."
}
```

# Response statuses:
| HTTP Status Code           | Description          |
|----------------------------|----------------------|
| 200 OK                     | The request succeeded.
| 400 Bad Request            | Request body validation failed (e.g., missing or invalid fields, or conversion, encoding, or decoding errors). |
| 503 Service Unavailable    | Indeterminate status - the request can be retried.
| 500 Internal Server Error  | Any other errors, with description provided in the `detail` field.



## 2. Helper endpoint `POST /verifier/<sourceName>/<attestationType>/prepareRequestBody`
Returns ABI-encoded request data. This helper endpoint generates the ABI-encoded `requestBody`.

- Note: Currently, this endpoint only performs encoding. Verification functionality will be added later.
### Request example for `TeeAvailabilityCheck`:
```json
{
  "attestationType": "0x546565417661696c6162696c697479436865636b000000000000000000000000",
  "sourceId": "0x7465650000000000000000000000000000000000000000000000000000000000",
  "requestData": {
    "challenge": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "teeId": "0x000000000000000000000000000000000000dEaD",
    "url": "https://proxy.url/tee"
  }
}
```
### Response example for `TeeAvailabilityCheck`:
```json
{
  "responseBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"
}
```

## 3. Helper endpoint `POST /verifier/<sourceName>/<attestationType>/prepareResponseBody`
Verify the encoded request body and returns both the decoded response data and its ABI-encoded form.
### Request example for `TeeAvailabilityCheck`:
```json
{
  "attestationType": "0x546565417661696c6162696c697479436865636b000000000000000000000000",
  "sourceID": "0x7465650000000000000000000000000000000000000000000000000000000000",
  "requestBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000dead00000000000000000000000000000000000000000000000000000000000000601234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef000000000000000000000000000000000000000000000000000000000000001668747470733a2f2f73757065727465652e70726f787900000000000000000000"
}
```
### Response example for `TeeAvailabilityCheck`:
```json
{
  "responseData": {
    "status": 0,
    "teeTimestamp": 123456789,
    "codeHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "platform": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "initialSigningPolicyId": 1,
    "lastSigningPolicyId": 2,
    "state": {
      "systemState": "0x0000000000000000000000000000000000000000000000000001231231230000000000000",
      "systemStateVersion": "0x0000000000000000000000000000000000000000000000000000000000000123",
      "state": "0x000000000000000000000000000000000000000000000000000123412340000000000",
      "stateVersion" : "0x0000000000000000000000000000000000000000000000000000001230000000",
    }
  },
  "responseBody": "0x0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
}
```


# Data Structures

- Common request with shared metadata.
```go
type AttestationRequest struct {
  AttestationType [32]byte
  SourceID        [32]byte
  RequestBody     []byte
}
```
| Field              | Description           |
|--------------------|-----------------------|
| AttestationType    | Hex-encoded 32-byte identifier of the attestation type
| SourceID           | Hex-encoded 32-byte source identifier
| RequestBody        | ABI encoded request data

- Attestations type `TeeAvailabilityCheck`:
```go
type TeeAvailabilityCheckRequestBody struct {
	TeeID     [20]byte
	URL       string
	Challenge [32]byte
}
```
| Field    | Description          |
|----------|----------------------|
| TeeID    | Hex-encoded 20-byte Ethereum address of the TEE
| URL      | TEE proxy endpoint URL
| Challenge| Hex-encoded 32-byte challenge

```go
type TeeAvailabilityCheckResponseBody struct {
	Status                 uint8
	TeeTimestamp           uint64
	CodeHash               [32]byte
	Platform               [32]byte
	InitialSigningPolicyID uint32
	LastSigningPolicyID    uint32
	State                  TeeAvailabilityCheckTeeState
}
```
```go
type TeeAvailabilityCheckTeeState struct {
	SystemState        []byte
	SystemStateVersion [32]byte
	State              []byte
	StateVersion       [32]byte
}
```
| Field                  | Description          |
|------------------------|----------------------|
| Status                 | Enum AvailabilityCheckStatus { OK, OBSOLETE, DOWN }
| TeeTimestamp           | Timestamp reported by the TEE
| CodeHash               | Hex-encoded 32-byte SHA-256 digest of the workload container image (from JWT)
| Platform               | Hex-encoded 32-byte hwmodel (from JWT)
| InitialSigningPolicyId | ID of the initial signing policy
| LastSigningPolicyId    | ID of the last signing policy
| State                  | Current TEE state

- Attestation type `PMWPaymentStatus`.
```go
type PMWPaymentStatusRequestBody struct {
	WalletID [32]byte
	Nonce    uint64
	SubNonce uint64
}
```
| Field    | Description
|----------|----------------------|
| WalletID | Unique wallet identifier
| Nonce    | Batch nonce of the payment instruction
| SubNonce | Sequence number of the payment instruction

```go
type IPMWPaymentStatusResponseBody struct {
	SenderAddress     string
	RecipientAddress  string
	Amount            *big.Int
	Fee               *big.Int
	PaymentReference  [32]byte
	TransactionStatus uint8
	RevertReason      string
	ReceivedAmount    *big.Int
	TransactionFee    *big.Int
	TransactionId     [32]byte
	BlockNumber       uint64
	BlockTimestamp    uint64
}
```
| Field             | Description          |
|-------------------|----------------------|
| SenderAddress     | Sender from the payment instruction message
| RecipientAddress  | Recipient from the payment instruction message
| Amount            | Amount from the payment instruction message
| Fee               | Fee from the payment instruction message
| PaymentReference  | Payment reference from the payment instruction message
| TransactionStatus | Enum 	TransactionStatus { Success, SenderFault, ReceiverFault }
| RevertReason      | Reason for transaction failure (blockchain-specific)
| ReceivedAmount    | Actual amount received by the recipient
| TransactionFee    | Total transaction fee spent
| TransactionId     | Transaction hash
| BlockNumber       | Block number in which the transaction was included
| BlockTimestamp    | Timestamp of the block containing the transaction


- Attestation type `PMWMultisigAccountConfigured` (a very specific attestation type, currently designed for XRPL).

```go
type IPMWMultisigAccountConfiguredRequestBody struct {
	AccountAddress string
	PublicKeys    [][]byte
	Threshold     uint64
}
```
| Field          | Description          |
|----------------|----------------------|
| AccountAddress | Address of the multisig account
| PublicKeys     | Public keys associated with multisig account
| Threshold      | Multisig threshold of the account

```go
type IPMWMultisigAccountConfiguredResponseBody struct {
	Status   uint8
	Sequence uint64
}
```
| Field    | Description          |
|----------|----------------------|
| Status   | Enum PMWMultisigAccountStatus { OK, ERROR }
| Sequence | Current sequence number of the account


## 4. Health endpoint `GET /api/health`

Returns
```json
{
  "healthy": true
}
```