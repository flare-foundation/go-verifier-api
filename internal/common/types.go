package types

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

type SourceName string

const (
	SourceTEE SourceName = "tee"
	SourceXRP SourceName = "xrp"
)

type AttestationType string

const (
	TeeAvailabilityCheck AttestationType = "TeeAvailabilityCheck"
	PMWPaymentStatus     AttestationType = "PMWPaymentStatus"
)
