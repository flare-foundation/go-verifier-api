package attestationtypes

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

type FTDCHeader struct {
	AttestationType string `json:"attestationType" validate:"required,hash32" example:"0x504d574d756c74697369674163636f756e74436f6e6669677572656400000000"`
	SourceId        string `json:"sourceId" validate:"required,hash32" example:"0x7465737478727000000000000000000000000000000000000000000000000000"`
	ThresholdBIPS   uint16 `json:"thresholdBIPS" example:"0"`
}

type FTDCRequest[T any] struct {
	FTDCHeader  FTDCHeader `json:"header"`
	RequestData T          `json:"requestData"`
}

type FTDCRequestEncoded struct {
	FTDCHeader  FTDCHeader `json:"header"`
	RequestBody string     `json:"responseBody" example:"0x0000abcd..."`
}

type EncodedRequestBody struct {
	RequestBody string `json:"requestBody" example:"0x0000abcd..."`
}

// TODO Common types for verifier and relay client.
type EncodedResponseBody struct {
	Response hexutil.Bytes `json:"Response" example:"0x0000abcd..."`
}

// Response is a generic response type for the API with just a simple body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
type Response[T any] struct {
	Body T
}

// NewResponse returns the response type with the right body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
func NewResponse[T any](body T) *Response[T] {
	return &Response[T]{Body: body}
}

func GetVerifierAPIPath(sourceName config.SourceName, attestationType connector.AttestationType, endpoint string) string {
	return fmt.Sprintf("/verifier/%s/%s/%s", strings.ToLower(string(sourceName)), attestationType, endpoint)
}

func GetVerifierAPITag(attestationType connector.AttestationType) []string {
	return []string{string(attestationType)}
}
