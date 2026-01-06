package types

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-playground/validator/v10"
)

type HealthCheckResponse struct {
	Healthy bool `json:"healthy"`
}

// Main API types.
type AttestationRequest struct {
	AttestationType common.Hash   `json:"attestationType" validate:"required" example:"0x504d574..."`
	SourceID        common.Hash   `json:"sourceId" validate:"required" example:"0x7465..."`
	RequestBody     hexutil.Bytes `json:"requestBody" validate:"required" example:"0x0000abcd..."`
}

// Resolve adds extra validation beyond struct tags, ensuring RequestBody has data.
func (req AttestationRequest) Resolve(ctx huma.Context) []error {
	if len(req.RequestBody) == 0 {
		return []error{fmt.Errorf("requestBody cannot be empty")}
	}
	return nil
}

type AttestationResponse struct {
	ResponseBody hexutil.Bytes `json:"responseBody" example:"0x0000abcd..."`
}

// Helper API types.
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

// Response is a generic response type for the API with just a simple body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
type Response[T any] struct {
	Body T
}

// NewResponse returns the response type with the right body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
func NewResponse[T any](body T) *Response[T] {
	return &Response[T]{Body: body}
}

// RequestConvertible defines an interface for requests that can be converted to internal ones.
type RequestConvertible[T any] interface {
	ToInternal() (T, error)
}

// InternalConvertible defines an interface for response that can be converted from internal ones.
type ResponseConvertible[T any] interface {
	// New() ExternalConvertible[T]
	FromInternal(T) ResponseConvertible[T]
	Log()
}
