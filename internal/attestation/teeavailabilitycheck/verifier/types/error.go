package types

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	ErrNetwork      = errors.New("network error")
	ErrRPC          = errors.New("rpc error")
	ErrInvalidInput = errors.New("invalid input")
	ErrContext      = errors.New("context error")
	ErrUnknown      = errors.New("unknown error")
)

// ClassifyHTTPStatus maps an HTTP status code to a TeeSampleState. Shared by
// both RPC and HTTP fetch paths so HTTP status semantics stay consistent:
// - 400/404 → INVALID (deterministic client error)
// - any other non-2xx → INDETERMINATE (transient server/proxy issue)
func ClassifyHTTPStatus(code int) (TeeSampleState, error) {
	switch code {
	case http.StatusBadRequest, http.StatusNotFound:
		return TeeSampleInvalid, ErrInvalidInput
	default:
		return TeeSampleIndeterminate, ErrNetwork
	}
}

// MapTransportError classifies transport-layer errors common to both RPC and
// HTTP fetch paths: context cancellation/deadline and network-level errors
// (DNS, TCP reset, TLS handshake, connection refused). HTTP status errors are
// handled per-domain by the caller because rpc.HTTPError and
// fetcher.HTTPStatusError have incompatible shapes (field vs method).
// Returns ok=false if the error is not a shared transport error.
func MapTransportError(err error) (state TeeSampleState, classified error, ok bool) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return TeeSampleIndeterminate, ErrContext, true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return TeeSampleIndeterminate, ErrNetwork, true
	}
	return TeeSampleValid, nil, false
}

func MapFetchErrorToState(op string, err error) (TeeSampleState, error) {
	wrapErr := func(e error) error {
		return &FetchError{Op: op, Err: e}
	}
	// Shared transport layer (context, net.Error).
	if state, classified, ok := MapTransportError(err); ok {
		return state, wrapErr(classified)
	}
	// Deterministic "not found" from go-ethereum (e.g. BlockByHash of a nonexistent block)
	// indicates invalid input, not a transport problem.
	if errors.Is(err, ethereum.NotFound) {
		return TeeSampleInvalid, wrapErr(ErrInvalidInput)
	}
	// HTTP layer (non-200 responses from RPC)
	var httpErr rpc.HTTPError
	if errors.As(err, &httpErr) {
		state, classified := ClassifyHTTPStatus(httpErr.StatusCode)
		return state, wrapErr(classified)
	}
	// JSON-RPC structured errors
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) {
		switch rpcErr.ErrorCode() {
		// Deterministic client-side issues → invalid
		case -32600, // invalid request
			-32601, // method not found
			-32602, // invalid params
			-32700: // parse error
			return TeeSampleInvalid, wrapErr(ErrInvalidInput)
		// Infra/server-side issues → indeterminate
		case -32000, // generic server error,
			-32002, // timeout
			-32003, // response too large
			-32603: // internal error
			return TeeSampleIndeterminate, wrapErr(ErrRPC)
		default:
			return TeeSampleIndeterminate, wrapErr(ErrRPC)
		}
	}
	return TeeSampleIndeterminate, wrapErr(ErrUnknown)
}

type FetchError struct {
	Op  string
	Err error
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *FetchError) Unwrap() error {
	return e.Err
}
