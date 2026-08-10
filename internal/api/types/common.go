package types

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-playground/validator/v10"
)

type HealthCheckResponse struct {
	Healthy bool `json:"healthy"`
}

// AttestationRequest is the main attestation request type.
type AttestationRequest struct {
	AttestationType common.Hash   `json:"attestationType" validate:"required" example:"0x504d574..."`
	SourceID        common.Hash   `json:"sourceId" validate:"required" example:"0x7465..."`
	RequestBody     hexutil.Bytes `json:"requestBody" validate:"required" example:"0x0000abcd..."`
}

// Resolve adds extra validation beyond struct tags, ensuring RequestBody has data.
func (req AttestationRequest) Resolve(ctx huma.Context) []error {
	if len(req.RequestBody) == 0 {
		return []error{errors.New("requestBody cannot be empty")}
	}
	return nil
}

// Verifier status values for the status-based /verify response envelope consumed
// by tee-relay-client. Any other value is a protocol violation on the wire.
const (
	StatusVerified = "VERIFIED"
	StatusRetry    = "RETRY"
	StatusRejected = "REJECTED"
)

// VerifierResponse is the /verify response envelope. Status is the verdict;
// ResponseBody is nonempty iff Status is VERIFIED; Message carries a safe reason
// when Status is not VERIFIED (RETRY = transient/retryable, REJECTED = terminal).
type VerifierResponse struct {
	Status       string        `json:"status" example:"VERIFIED"`
	ResponseBody hexutil.Bytes `json:"responseBody,omitempty" example:"0x0000abcd..."`
	Message      string        `json:"message,omitempty"`
}

// AttestationRequestData is a generic request type with decoded request data.
type AttestationRequestData[T any] struct {
	AttestationType common.Hash `json:"attestationType" validate:"required" example:"0x504d574..."`
	SourceID        common.Hash `json:"sourceId" validate:"required" example:"0x7465..."`
	RequestData     T           `json:"requestData" validate:"required"`
}

func (req AttestationRequestData[T]) Resolve(ctx huma.Context) []error {
	var errs []error
	if valErr := validator.New().Struct(req.RequestData); valErr != nil {
		errs = append(errs, valErr)
	}
	return errs
}

type AttestationRequestEncoded struct {
	RequestBody hexutil.Bytes `json:"requestBody" example:"0x0000abcd..."`
}

type AttestationResponseData[T any] struct {
	ResponseData T             `json:"responseData"`
	ResponseBody hexutil.Bytes `json:"responseBody" example:"0x0000abcd..."`
}

// Response is a generic API wrapper that holds a response body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
type Response[T any] struct {
	Body T
}

// NewResponse wraps the body into the response envelope. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
func NewResponse[T any](body T) *Response[T] {
	return &Response[T]{Body: body}
}

// RequestConvertible defines an interface for requests that can be converted to internal ones.
type RequestConvertible[T any] interface {
	ToInternal() (T, error)
}

// ResponseConvertible defines an interface for response that can be converted from internal ones.
type ResponseConvertible[T any] interface {
	FromInternal(T) ResponseConvertible[T]
	Log()
}
