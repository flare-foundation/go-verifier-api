package coreutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/ethereum/go-ethereum/rpc"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
)

var (
	ErrNotFound     = errors.New("resource not found (404)")
	ErrNetwork      = errors.New("network error")
	ErrRPC          = errors.New("rpc error")
	ErrInvalidInput = errors.New("invalid input")
	ErrContext      = errors.New("context error")
	ErrUnknown      = errors.New("unknown error")
)

func MapFetchErrorToState(op string, err error) (teetype.TeePollerSampleState, error) {
	wrapErr := func(e error) error {
		return &FetchError{Op: op, Err: e}
	}
	// Context issues
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return teetype.TeePollerSampleIndeterminate, wrapErr(ErrContext)
	}
	// HTTP layer (non-200 responses)
	var httpErr rpc.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusBadRequest, http.StatusNotFound:
			return teetype.TeePollerSampleInvalid, wrapErr(ErrInvalidInput)
		default:
			return teetype.TeePollerSampleIndeterminate, wrapErr(ErrNetwork)
		}
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
			return teetype.TeePollerSampleInvalid, wrapErr(ErrInvalidInput)
		// Infra/server-side issues → indeterminate
		case -32000, // generic server error,
			-32002, // timeout
			-32003, // response too large
			-32603: // internal error
			return teetype.TeePollerSampleIndeterminate, wrapErr(ErrRPC)
		default:
			return teetype.TeePollerSampleIndeterminate, wrapErr(ErrRPC)
		}
	}
	// Network issues (DNS fail, conn refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return teetype.TeePollerSampleIndeterminate, wrapErr(ErrNetwork)
	}
	return teetype.TeePollerSampleIndeterminate, wrapErr(ErrUnknown)
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
