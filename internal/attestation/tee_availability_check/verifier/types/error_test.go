package types_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/fetcher"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
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

func TestFetchJSON(t *testing.T) {
	type testStruct struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}

	// Set up a temporary HTTP server
	handler := http.NewServeMux()
	handler.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"foo":"hello","bar":42}`))
	})
	handler.HandleFunc("/notfound", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler.HandleFunc("/badjson", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"foo":123,"bar":"0x123"}`)) // invalid types
	})
	handler.HandleFunc("/unexpected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
		_, _ = w.Write([]byte(`{"foo":"irrelevant","bar":0}`))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		result, err := fetcher.GetJSON[testStruct](ctx, fmt.Sprintf("%s/ok", server.URL), 50*time.Millisecond)
		require.NoError(t, err)
		require.Equal(t, testStruct{Foo: "hello", Bar: 42}, result)
	})
	t.Run("not found", func(t *testing.T) {
		ctx := context.Background()
		_, err := fetcher.GetJSON[testStruct](ctx, fmt.Sprintf("%s/notfound", server.URL), 50*time.Millisecond)
		require.ErrorIs(t, err, verifiertypes.ErrNotFound)
	})
	t.Run("unexpected status code", func(t *testing.T) {
		ctx := context.Background()
		_, err := fetcher.GetJSON[testStruct](ctx, fmt.Sprintf("%s/unexpected", server.URL), 50*time.Millisecond)
		require.ErrorContains(t, err, "unexpected status code")
	})
	t.Run("bad json", func(t *testing.T) {
		ctx := context.Background()
		_, err := fetcher.GetJSON[testStruct](ctx, fmt.Sprintf("%s/badjson", server.URL), 50*time.Millisecond)
		require.ErrorContains(t, err, "decoding JSON from http://127.0.0.1")
		require.ErrorContains(t, err, "failed for type types_test.testStruct")
	})
	t.Run("timeout", func(t *testing.T) {
		slowHandler := http.NewServeMux()
		slowHandler.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"foo":"late","bar":1}`))
		})
		slowServer := httptest.NewServer(slowHandler)
		defer slowServer.Close()

		ctx := context.Background()
		_, err := fetcher.GetJSON[testStruct](ctx, fmt.Sprintf("%s/slow", slowServer.URL), 50*time.Millisecond)
		require.Contains(t, err.Error(), "context deadline exceeded")
	})
	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := fetcher.GetJSON[testStruct](ctx, fmt.Sprintf("%s/ok", server.URL), 50*time.Millisecond)
		require.ErrorContains(t, err, "context canceled")
	})
	t.Run("malformed URL causes request creation failure", func(t *testing.T) {
		ctx := context.Background()
		_, err := fetcher.GetJSON[testStruct](ctx, "http://%41:8080", 50*time.Millisecond)
		require.ErrorContains(t, err, "failed to create HTTP request")
	})
	t.Run("FetchError methods", func(t *testing.T) {
		err := &verifiertypes.FetchError{Op: "fetchOp", Err: verifiertypes.ErrRPC}
		require.Contains(t, err.Error(), "fetchOp")
		require.Contains(t, err.Error(), verifiertypes.ErrRPC.Error())
		require.ErrorIs(t, err, verifiertypes.ErrRPC)
	})
	t.Run("oversized json", func(t *testing.T) {
		bigData := []byte(`{"foo":"`)
		garbage := make([]byte, 3*1024*1024)
		for i := range garbage {
			garbage[i] = 'x'
		}
		bigData = append(bigData, garbage...)
		bigData = append(bigData, []byte(`"}`)...)

		bigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bigData)
		}))
		defer bigServer.Close()

		ctx := context.Background()
		_, err := fetcher.GetJSON[testStruct](ctx, bigServer.URL, 1*time.Second)

		require.Error(t, err)
		require.Contains(t, err.Error(), "decoding JSON from")
	})
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
