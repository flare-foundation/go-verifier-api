package attestationtypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type HealthCheckResponse struct {
	Healthy bool `json:"healthy"`
}

// Main API types.
type AttestationRequest struct {
	AttestationType common.Hash   `json:"attestationType" validate:"required,hash32" example:"0x504d574..."`
	SourceID        common.Hash   `json:"sourceID" validate:"required,hash32" example:"0x7465..."`
	RequestBody     hexutil.Bytes `json:"requestBody" example:"0x0000abcd..."`
}

type AttestationResponse struct {
	ResponseBody hexutil.Bytes `json:"responseBody" example:"0x0000abcd..."`
}

// Helper API types.
type AttestationRequestData[T any] struct {
	AttestationType common.Hash `json:"attestationType" validate:"required,hash32" example:"0x504d574..."`
	SourceID        common.Hash `json:"sourceID" validate:"required,hash32" example:"0x7465..."`
	RequestData     T           `json:"requestData"`
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
