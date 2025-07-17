package attestationtypes

type SourceName string

const (
	SourceTEE SourceName = "tee"
	SourceXRP SourceName = "xrp"
)

type EncodedRequestBody struct {
	EncodedRequestBody string `json:"encodedRequestBody" example:"0x0000abcd..."`
}

type EncodedResponseBody struct {
	EncodedResponseBody string `json:"encodedResponseBody" example:"0x0000abcd..."`
}

// Response is a generic response type for the API with just a simple body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
type Response[T any] struct {
	Body T
}

// NewResponse returns the response type with the right body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
func NewResponse[T any](body T) *Response[T] {
	return &Response[T]{Body: body}
}
