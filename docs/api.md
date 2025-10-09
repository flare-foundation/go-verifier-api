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
  "attestationType": "0x504...",
  "sourceId": "0x746...",
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
| 503 Service Unavailable    | Indeterminate status - the request can be retried. This occurs when there is no result for the provided challenge and the poller has only VALID entries for that TEE.
| 500 Internal Server Error  | Any other errors, with description provided in the `detail` field.



## 2. Helper endpoint `POST /verifier/<sourceName>/<attestationType>/prepareRequestBody`
Returns ABI-encoded request data. This helper endpoint generates the ABI-encoded `requestBody`.

> **NOTE**: Currently, this endpoint only performs encoding.
### Example for `PMWMultisigAccountConfigured`:
Request:
```json
{
  "attestationType": "0x504d574d756c74697369674163636f756e74436f6e6669677572656400000000",
  "sourceId": "0x7465737458525000000000000000000000000000000000000000000000000000",
  "requestData": {
    "accountAddress": "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
    "publicKeys": [
      "0x51003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240",
      "0x06276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f",
      "0x76e4a85207c1012283a7190b1df628e29ba1a687404ec35a766e7eddba94ba42a07f356ccc847540b4ed23f15f3feb07c406c3f815a361983c321740fa998cdb"
    ],
    "threshold": 1
  }
}
```
Response:
```json
{
  "requestBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000022724d44437253596265476d373761596a6e767548566e42775a31546b4c6e7531554c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c00000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000004051003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240000000000000000000000000000000000000000000000000000000000000004006276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f000000000000000000000000000000000000000000000000000000000000004076e4a85207c1012283a7190b1df628e29ba1a687404ec35a766e7eddba94ba42a07f356ccc847540b4ed23f15f3feb07c406c3f815a361983c321740fa998cdb"
}
```

## 3. Helper endpoint `POST /verifier/<sourceName>/<attestationType>/prepareResponseBody`
Verify the encoded request body and returns both the decoded response data and its ABI-encoded form.
### Example for `PMWMultisigAccountConfigured`:
Request:
```json
{
  "attestationType": "0x504d574d756c74697369674163636f756e74436f6e6669677572656400000000",
  "sourceId": "0x7465737458525000000000000000000000000000000000000000000000000000",
  "requestBody": "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000022724d44437253596265476d373761596a6e767548566e42775a31546b4c6e7531554c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c00000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000004051003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240000000000000000000000000000000000000000000000000000000000000004006276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f000000000000000000000000000000000000000000000000000000000000004076e4a85207c1012283a7190b1df628e29ba1a687404ec35a766e7eddba94ba42a07f356ccc847540b4ed23f15f3feb07c406c3f815a361983c321740fa998cdb"
}
```
Response:
```json
{
  "responseData": {
    "status": 0,
    "sequence": 10136106
  },
  "responseBody": "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000009aaa2a"
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
	TeeID      [20]byte
	ProxyTeeID [20]byte
	URL        string
	Challenge  [32]byte
}
```
| Field      | Description          |
|------------|----------------------|
| TeeID      | Hex-encoded 20-byte Ethereum address of the TEE
| ProxyTeeID | Hex-encoded 20-byte Ethereum address of the TEE Proxy ID
| URL        | TEE proxy endpoint URL
| Challenge  | Hex-encoded 32-byte challenge

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
| InitialSigningPolicyID | ID of the initial signing policy
| LastSigningPolicyID    | ID of the last signing policy
| State                  | Current TEE state

## Additional helper endpoint `GET /poller/tees`
Lists all TEEs currently being monitored by the poller along with their recent status samples.

Example response:
```json
{
  "samples": [
    {
      "tee_id": "0xb0351006C48162bB692f00Bfb988c6E622F7bD6f",
      "values": [
        {
          "timestamp": "2025-09-30T09:05:33.794872Z",
          "state": "VALID"
        },
		{
          "timestamp": "2025-09-30T09:06:33.794872Z",
          "state": "INVALID"
        },
		{
          "timestamp": "2025-09-30T09:07:33.794872Z",
          "state": "INDETERMINATE"
        }
      ]
    }
  ]
}
```

- Attestation type `PMWPaymentStatus`.
```go
type PMWPaymentStatusRequestBody struct {
	OpType 		  [32]byte
	SenderAddress string
	Nonce    	  uint64
	SubNonce 	  uint64
}
```
| Field    		| Description
|---------------|----------------------|
| OpType   		| Hex-encoded 32-byte operational type
| SenderAddress | Sender address
| Nonce    		| Batch nonce of the payment instruction
| SubNonce 		| Sequence number of the payment instruction

```go
type IPMWPaymentStatusResponseBody struct {
	RecipientAddress  string
	TokenId 		  [32]byte
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
| RecipientAddress  | Recipient from the payment instruction message
| TokenId			| Token ID (e.g. address) for the payment, bytes32(0) represents native token
| Amount            | Amount from the payment instruction message
| Fee               | Fee from the payment instruction message
| PaymentReference  | Payment reference from the payment instruction message
| TransactionStatus | Enum 	TransactionStatus { Success, Reverted }
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
Returns the current health status of the service. This is the only endpoint accessible without an API key.

Example response:
```json
{
  "healthy": true
}
```