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
	Response          AttestationResponse[Req, Res] `json:"response"`
}

type SourceName string

const (
	SourceTEE SourceName = "tee"
	SourceXRP SourceName = "xrp"
)

type AttestationResponseStatus string

const (
	VALID          AttestationResponseStatus = "VALID"
	INVALID        AttestationResponseStatus = "INVALID" // TODO -> check all INVALID statuses and substitute with appropriate one
	SYSTEM_FAILURE AttestationResponseStatus = "INDETERMINATE: SYSTEM_FAILURE"
)
