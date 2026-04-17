package types_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
	"github.com/stretchr/testify/require"
)

func TestMapFetchErrorToState(t *testing.T) {
	t.Run("context errors", func(t *testing.T) {
		st, err := verifiertypes.MapFetchErrorToState("op1", context.DeadlineExceeded)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, st)
		requireFetchError(t, err, "op1", verifiertypes.ErrContext)

		st, err = verifiertypes.MapFetchErrorToState("op2", context.Canceled)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, st)
		requireFetchError(t, err, "op2", verifiertypes.ErrContext)
	})
	t.Run("ethereum not found is invalid input", func(t *testing.T) {
		st, err := verifiertypes.MapFetchErrorToState("fetch block", ethereum.NotFound)
		require.Equal(t, verifiertypes.TeeSampleInvalid, st)
		requireFetchError(t, err, "fetch block", verifiertypes.ErrInvalidInput)

		// Wrapped not-found also classifies as invalid.
		wrapped := fmt.Errorf("chain call failed: %w", ethereum.NotFound)
		st, err = verifiertypes.MapFetchErrorToState("fetch block", wrapped)
		require.Equal(t, verifiertypes.TeeSampleInvalid, st)
		requireFetchError(t, err, "fetch block", verifiertypes.ErrInvalidInput)
	})
	t.Run("HTTP error", func(t *testing.T) {
		// Simulate HTTP 400 (BadRequest)
		httpErr := rpc.HTTPError{StatusCode: http.StatusBadRequest, Status: "Bad Request", Body: []byte("bad")}
		st, err := verifiertypes.MapFetchErrorToState("op", httpErr)
		require.Equal(t, verifiertypes.TeeSampleInvalid, st)
		requireFetchError(t, err, "op", verifiertypes.ErrInvalidInput)

		// Simulate HTTP 404 (NotFound)
		httpErr = rpc.HTTPError{StatusCode: http.StatusNotFound, Status: "Not Found", Body: []byte("not found")}
		st, err = verifiertypes.MapFetchErrorToState("op", httpErr)
		require.Equal(t, verifiertypes.TeeSampleInvalid, st)
		requireFetchError(t, err, "op", verifiertypes.ErrInvalidInput)

		// Simulate HTTP 500 (should be indeterminate/network)
		httpErr = rpc.HTTPError{StatusCode: http.StatusInternalServerError, Status: "Internal Server Error", Body: []byte("fail")}
		st, err = verifiertypes.MapFetchErrorToState("op", httpErr)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, st)
		requireFetchError(t, err, "op", verifiertypes.ErrNetwork)
	})
	t.Run("RPC error", func(t *testing.T) {
		// Deterministic client-side errors
		for _, code := range []int{-32600, -32601, -32602, -32700} {
			rpcErr := &testRPCError{code: code, message: "client error"}
			st, err := verifiertypes.MapFetchErrorToState("op", rpcErr)
			require.Equal(t, verifiertypes.TeeSampleInvalid, st, "code %d should be invalid", code)
			requireFetchError(t, err, "op", verifiertypes.ErrInvalidInput)
		}

		// Infra/server-side errors
		for _, code := range []int{-32000, -32002, -32003, -32603} {
			rpcErr := &testRPCError{code: code, message: "server error"}
			st, err := verifiertypes.MapFetchErrorToState("op", rpcErr)
			require.Equal(t, verifiertypes.TeeSampleIndeterminate, st, "code %d should be indeterminate", code)
			requireFetchError(t, err, "op", verifiertypes.ErrRPC)
		}

		// Unknown error code (should still be indeterminate/RPC)
		rpcErr := &testRPCError{code: -12345, message: "unknown"}
		st, err := verifiertypes.MapFetchErrorToState("op", rpcErr)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, st)
		requireFetchError(t, err, "op", verifiertypes.ErrRPC)
	})
	t.Run("network error", func(t *testing.T) {
		// Simulate a net.Error (timeout)
		netErr := &testNetError{timeout: true, temporary: false, msg: "timeout"}
		st, err := verifiertypes.MapFetchErrorToState("op", netErr)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, st)
		requireFetchError(t, err, "op", verifiertypes.ErrNetwork)

		// Simulate a net.Error (temporary)
		netErr = &testNetError{timeout: false, temporary: true, msg: "temporary"}
		st, err = verifiertypes.MapFetchErrorToState("op", netErr)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, st)
		requireFetchError(t, err, "op", verifiertypes.ErrNetwork)
	})
	t.Run("unknown error", func(t *testing.T) {
		unknownErr := errors.New("unknown error")
		st, err := verifiertypes.MapFetchErrorToState("op", unknownErr)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, st)
		requireFetchError(t, err, "op", verifiertypes.ErrUnknown)
	})
}

func TestMapTransportError(t *testing.T) {
	t.Run("context.DeadlineExceeded", func(t *testing.T) {
		state, classified, ok := verifiertypes.MapTransportError(context.DeadlineExceeded)
		require.True(t, ok)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		require.ErrorIs(t, classified, verifiertypes.ErrContext)
	})
	t.Run("context.Canceled", func(t *testing.T) {
		state, classified, ok := verifiertypes.MapTransportError(context.Canceled)
		require.True(t, ok)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		require.ErrorIs(t, classified, verifiertypes.ErrContext)
	})
	t.Run("net.Error", func(t *testing.T) {
		netErr := &testNetError{timeout: true, msg: "timeout"}
		state, classified, ok := verifiertypes.MapTransportError(netErr)
		require.True(t, ok)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		require.ErrorIs(t, classified, verifiertypes.ErrNetwork)
	})
	t.Run("unrelated error returns ok=false", func(t *testing.T) {
		_, _, ok := verifiertypes.MapTransportError(errors.New("not a transport error"))
		require.False(t, ok)
	})
}

func TestClassifyHTTPStatus(t *testing.T) {
	t.Run("400 is invalid", func(t *testing.T) {
		state, classified := verifiertypes.ClassifyHTTPStatus(http.StatusBadRequest)
		require.Equal(t, verifiertypes.TeeSampleInvalid, state)
		require.ErrorIs(t, classified, verifiertypes.ErrInvalidInput)
	})
	t.Run("404 is invalid", func(t *testing.T) {
		state, classified := verifiertypes.ClassifyHTTPStatus(http.StatusNotFound)
		require.Equal(t, verifiertypes.TeeSampleInvalid, state)
		require.ErrorIs(t, classified, verifiertypes.ErrInvalidInput)
	})
	t.Run("500 is indeterminate", func(t *testing.T) {
		state, classified := verifiertypes.ClassifyHTTPStatus(http.StatusInternalServerError)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		require.ErrorIs(t, classified, verifiertypes.ErrNetwork)
	})
	t.Run("502 is indeterminate", func(t *testing.T) {
		state, classified := verifiertypes.ClassifyHTTPStatus(http.StatusBadGateway)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		require.ErrorIs(t, classified, verifiertypes.ErrNetwork)
	})
}

func TestFetchError(t *testing.T) {
	err := &verifiertypes.FetchError{Op: "fetchOp", Err: verifiertypes.ErrRPC}
	require.Contains(t, err.Error(), "fetchOp")
	require.Contains(t, err.Error(), verifiertypes.ErrRPC.Error())
	require.ErrorIs(t, err, verifiertypes.ErrRPC)
}

type testRPCError struct {
	code    int
	message string
}

func (e *testRPCError) Error() string  { return e.message }
func (e *testRPCError) ErrorCode() int { return e.code }

type testNetError struct {
	timeout   bool
	temporary bool
	msg       string
}

func (e *testNetError) Error() string   { return e.msg }
func (e *testNetError) Timeout() bool   { return e.timeout }
func (e *testNetError) Temporary() bool { return e.temporary }

func requireFetchError(t *testing.T, err error, wantOp string, wantErr error) {
	t.Helper()
	var fetchErr *verifiertypes.FetchError
	require.ErrorAs(t, err, &fetchErr, "error should be of type *FetchError")
	require.Equal(t, wantOp, fetchErr.Op, "FetchError.Op mismatch")
	require.ErrorIs(t, fetchErr.Err, wantErr, "FetchError.Err mismatch")
}
