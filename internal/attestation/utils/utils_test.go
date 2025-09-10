package utils

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/types"
	testutil "github.com/flare-foundation/go-verifier-api/internal/test_util"

	"github.com/stretchr/testify/require"
)

func TestBytes32(t *testing.T) {
	s := "short"
	b, err := Bytes32(s)
	require.NoError(t, err, "unexpected error: %v", err)
	require.Equal(t, s, string(b[:len(s)]), "expected prefix %q, got %q", s, string(b[:len(s)]))

	long := strings.Repeat("a", Bytes32Size+1)
	_, err = Bytes32(long)
	require.Error(t, err)
}

func TestBytesToHex0x(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	got := BytesToHex0x(data)
	require.Equal(t, "0x010203", got)
}

func TestRemoveHexPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"0xabc", "abc"},
		{"0Xabc", "abc"},
		{"abc", "abc"},
	}
	for _, tt := range tests {
		got := RemoveHexPrefix(tt.in)
		require.Equal(t, tt.want, got, "RemoveHexPrefix(%q) = %q, want %q", tt.in, got, tt.want)
	}
}

func TestHexStringToBytes32(t *testing.T) {
	tests := []testutil.TestCase[string, string]{
		{
			Input:         "0x" + strings.Repeat("a1", 32),
			ExpectError:   false,
			ExpectedValue: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
		},
		{
			Input:          "0x1234",
			ExpectError:    true,
			ExpectedErrMsg: "invalid length for bytes32: got 2 bytes, expected 32",
		},
		{
			Input:          "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			ExpectError:    true,
			ExpectedErrMsg: "encoding/hex: invalid byte: U+007A 'z'",
		},
		{
			Input:          "0x" + strings.Repeat("ff", 33), // 66 hex chars = 33 bytes, too long
			ExpectError:    true,
			ExpectedErrMsg: "invalid length for bytes32: got 33 bytes, expected 32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.TestName, func(t *testing.T) {
			h, err := HexStringToBytes32(tt.Input)
			if tt.ExpectError {
				require.Error(t, err, "expected error for input %q", tt.Input)
				require.Equal(t, tt.ExpectedErrMsg, err.Error())
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
				require.Equal(t, tt.ExpectedValue, hex.EncodeToString(h[:]))
			}
		})
	}
}

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	want := 42
	op := func() (int, error) {
		return want, nil
	}
	got, err := Retry(3, time.Millisecond, op, nil)
	require.NoError(t, err, "expected no error")
	require.Equal(t, want, got, "expected %d, got %d", want, got)
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	op := func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporary failure")
		}
		return "ok", nil
	}
	got, err := Retry(5, time.Millisecond, op, nil)
	require.NoError(t, err, "expected success")
	require.Equal(t, "ok", got, "expected ok, got %s", got)
	require.Equal(t, 3, attempts, "expected 3 attempts, got %d", attempts)
}

func TestRetry_ExhaustRetries(t *testing.T) {
	op := func() (int, error) {
		return 0, errors.New("always fails")
	}
	_, err := Retry(3, time.Millisecond, op, nil)
	require.Error(t, err, "expected error, got nil")
}

func TestRetry_BreakOn(t *testing.T) {
	specialErr := errors.New("stop now")
	attempts := 0
	op := func() (string, error) {
		attempts++
		if attempts == 2 {
			return "bad", specialErr
		}
		return "", errors.New("regular failure")
	}
	got, err := Retry(5, time.Millisecond, op, func(e error) bool {
		return errors.Is(e, specialErr)
	})
	require.ErrorIs(t, err, specialErr, "expected specialErr, got %v", err)
	require.Equal(t, "bad", got, "expected bad, got %s", got)
	require.Equal(t, 2, attempts, "expected 2 attempts, got %d", attempts)
}

func TestRetry_ReturnsLastResult(t *testing.T) {
	op := func() (int, error) {
		return 99, errors.New("fail but keep result")
	}
	got, err := Retry(2, time.Millisecond, op, nil)
	require.Error(t, err)
	require.Equal(t, 99, got)
}

func TestClassifyFetchError(t *testing.T) {
	t.Run("Context errors", func(t *testing.T) {
		st, err := ClassifyFetchError("op1", context.DeadlineExceeded)
		require.Equal(t, teetypes.TeePollerSampleIndeterminate, st)
		requireFetchError(t, err, "op1", ErrContext)

		st, err = ClassifyFetchError("op2", context.Canceled)
		require.Equal(t, teetypes.TeePollerSampleIndeterminate, st)
		requireFetchError(t, err, "op2", ErrContext)
	})

	t.Run("HTTPError", func(t *testing.T) {
		// Simulate HTTP 400 (BadRequest)
		httpErr := rpc.HTTPError{StatusCode: http.StatusBadRequest, Status: "Bad Request", Body: []byte("bad")}
		st, err := ClassifyFetchError("op", httpErr)
		require.Equal(t, teetypes.TeePollerSampleInvalid, st)
		requireFetchError(t, err, "op", ErrInvalidInput)

		// Simulate HTTP 404 (NotFound)
		httpErr = rpc.HTTPError{StatusCode: http.StatusNotFound, Status: "Not Found", Body: []byte("not found")}
		st, err = ClassifyFetchError("op", httpErr)
		require.Equal(t, teetypes.TeePollerSampleInvalid, st)
		requireFetchError(t, err, "op", ErrInvalidInput)

		// Simulate HTTP 500 (should be indeterminate/network)
		httpErr = rpc.HTTPError{StatusCode: http.StatusInternalServerError, Status: "Internal Server Error", Body: []byte("fail")}
		st, err = ClassifyFetchError("op", httpErr)
		require.Equal(t, teetypes.TeePollerSampleIndeterminate, st)
		requireFetchError(t, err, "op", ErrNetwork)
	})

	t.Run("RPCErrors", func(t *testing.T) {
		// Deterministic client-side errors
		for _, code := range []int{-32600, -32601, -32602, -32700} {
			rpcErr := &testRPCError{code: code, message: "client error"}
			st, err := ClassifyFetchError("op", rpcErr)
			require.Equal(t, teetypes.TeePollerSampleInvalid, st, "code %d should be invalid", code)
			requireFetchError(t, err, "op", ErrInvalidInput)
		}

		// Infra/server-side errors
		for _, code := range []int{-32000, -32002, -32003, -32603} {
			rpcErr := &testRPCError{code: code, message: "server error"}
			st, err := ClassifyFetchError("op", rpcErr)
			require.Equal(t, teetypes.TeePollerSampleIndeterminate, st, "code %d should be indeterminate", code)
			requireFetchError(t, err, "op", ErrRPC)
		}

		// Unknown error code (should still be indeterminate/RPC)
		rpcErr := &testRPCError{code: -12345, message: "unknown"}
		st, err := ClassifyFetchError("op", rpcErr)
		require.Equal(t, teetypes.TeePollerSampleIndeterminate, st)
		requireFetchError(t, err, "op", ErrRPC)
	})

	t.Run("NetworkError", func(t *testing.T) {
		// Simulate a net.Error (timeout)
		netErr := &testNetError{timeout: true, temporary: false, msg: "timeout"}
		st, err := ClassifyFetchError("op", netErr)
		require.Equal(t, teetypes.TeePollerSampleIndeterminate, st)
		requireFetchError(t, err, "op", ErrNetwork)

		// Simulate a net.Error (temporary)
		netErr = &testNetError{timeout: false, temporary: true, msg: "temporary"}
		st, err = ClassifyFetchError("op", netErr)
		require.Equal(t, teetypes.TeePollerSampleIndeterminate, st)
		requireFetchError(t, err, "op", ErrNetwork)
	})

	t.Run("UnknownError", func(t *testing.T) {
		unknownErr := errors.New("unknown error")
		st, err := ClassifyFetchError("op", unknownErr)
		require.Equal(t, teetypes.TeePollerSampleIndeterminate, st)
		requireFetchError(t, err, "op", ErrUnknown)
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

	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		result, err := FetchJSON[testStruct](ctx, fmt.Sprintf("%s/ok", server.URL), 50*time.Millisecond)
		require.NoError(t, err)
		require.Equal(t, testStruct{Foo: "hello", Bar: 42}, result)
	})

	t.Run("Not found", func(t *testing.T) {
		ctx := context.Background()
		_, err := FetchJSON[testStruct](ctx, fmt.Sprintf("%s/notfound", server.URL), 50*time.Millisecond)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("Unexpected status code", func(t *testing.T) {
		ctx := context.Background()
		_, err := FetchJSON[testStruct](ctx, fmt.Sprintf("%s/unexpected", server.URL), 50*time.Millisecond)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected status code")
	})

	t.Run("Bad json", func(t *testing.T) {
		ctx := context.Background()
		_, err := FetchJSON[testStruct](ctx, fmt.Sprintf("%s/badjson", server.URL), 50*time.Millisecond)
		require.Error(t, err)
		require.Contains(t, err.Error(), "decoding failed")
	})

	t.Run("Request error", func(t *testing.T) {
		// Use an invalid URL to force a request error
		ctx := context.Background()
		_, err := FetchJSON[testStruct](ctx, "http://invalid.invalid/doesnotexist", 50*time.Millisecond)
		require.Error(t, err)
	})

	t.Run("Timeout", func(t *testing.T) {
		// Use a handler that sleeps longer than the timeout
		slowHandler := http.NewServeMux()
		slowHandler.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"foo":"late","bar":1}`))
		})
		slowServer := httptest.NewServer(slowHandler)
		defer slowServer.Close()

		ctx := context.Background()
		_, err := FetchJSON[testStruct](ctx, fmt.Sprintf("%s/slow", slowServer.URL), 50*time.Millisecond)
		require.Error(t, err)
	})

	t.Run("Context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := FetchJSON[testStruct](ctx, fmt.Sprintf("%s/ok", server.URL), 50*time.Millisecond)
		require.Error(t, err)
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
	var fetchErr *FetchError
	require.ErrorAs(t, err, &fetchErr, "error should be of type *FetchError")
	require.Equal(t, wantOp, fetchErr.Op, "FetchError.Op mismatch")
	require.ErrorIs(t, fetchErr.Err, wantErr, "FetchError.Err mismatch")
}
