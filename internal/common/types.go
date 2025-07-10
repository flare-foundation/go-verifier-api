package attestationtypes

type AttestationRequest[T any] struct {
	AttestationType string `json:"attestation_type"`
	SourceID        string `json:"source_id"`
	RequestBody     T      `json:"request_body"`
}

type AttestationResponse[Req any, Res any] struct {
	AttestationType string `json:"attestation_type"`
	SourceID        string `json:"source_id"`
	RequestBody     Req    `json:"request_body"`
	ResponseBody    Res    `json:"response_body"`
}

type FullAttestationResponse[Req any, Res any] struct {
	AttestationStatus AttestationResponseStatus     `json:"attestation_status"`
	Response          AttestationResponse[Req, Res] `json:"response,omitempty"`
}

type SourceName string

const (
	SourceTEE SourceName = "tee"
	SourceXRP SourceName = "xrp"
)

type AttestationResponseStatus string

const (
	VALID   AttestationResponseStatus = "VALID"
	INVALID AttestationResponseStatus = "INVALID" // TODO -> check all INVALID statuses and substitute with appropriate one
	// indeterminate
	SYSTEM_FAILURE AttestationResponseStatus = "INDETERMINATE: SYSTEM_FAILURE" // TODO unused for now
	// indeterminate TeeAvailabilityCheck
	CANNOT_LOAD_ROOT_CERTIFICATE     AttestationResponseStatus = "INDETERMINATE: CANNOT LOAD ROOT CERTIFICATE"
	CANNOT_FETCH_LAST_SIGNING_POLICY AttestationResponseStatus = "INDETERMINATE: CANNOT FETCH LAST SIGNING POLICY"
	INSUFFICIENT_POLLING_DATA        AttestationResponseStatus = "INDETERMINATE: INSUFFICIENT POLLING DATA"
	TEE_DATA_NOT_AVAILABLE           AttestationResponseStatus = "INDETERMINATE: TEE DATA NOT AVAILABLE"
	CANNOT_PARSE_CLAIMS              AttestationResponseStatus = "INDETERMINATE: CANNOT PARSE CLAIMS"
	// other failures TeeAvailabilityCheck
	NOT_IN_PRODUCTION_MODE         AttestationResponseStatus = "NOT IN PRODUCTION MODE"
	NOT_RUNNING_CONFIDENTIAL_SPACE AttestationResponseStatus = "NOT RUNNING CONFIDENTIAL SPACE"
	EAT_NONCE_MISSING              AttestationResponseStatus = "EAT NONCE MISSING"
	EAT_NONCE_MISMATCH             AttestationResponseStatus = "EAT NONCE MISMATCH"
	CERTIFICATE_CHECK_FAILED       AttestationResponseStatus = "CERTIFICATE CHECK FAILED"
	CERTIFICATE_INVALID            AttestationResponseStatus = "CERTIFICATE IS INVALID"
	LAST_SIGNING_POLICY_MISMATCH   AttestationResponseStatus = "LAST SIGNING POLICY MISMATCH"
)
