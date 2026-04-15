package teepoller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/fetcher"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
	"github.com/stretchr/testify/require"
)

func TestClassifyInfoFetchError(t *testing.T) {
	t.Run("nil error returns Valid", func(t *testing.T) {
		require.Equal(t, verifiertypes.TeeSampleValid, classifyInfoFetchError(nil))
	})

	t.Run("context deadline is Indeterminate", func(t *testing.T) {
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(context.DeadlineExceeded))
	})

	t.Run("context canceled is Indeterminate", func(t *testing.T) {
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(context.Canceled))
	})

	t.Run("wrapped context deadline is Indeterminate", func(t *testing.T) {
		err := fmt.Errorf("fetch failed: %w", context.DeadlineExceeded)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(err))
	})

	t.Run("net.Error is Indeterminate", func(t *testing.T) {
		// *net.OpError satisfies net.Error.
		netErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(netErr))
	})

	t.Run("wrapped net.Error is Indeterminate", func(t *testing.T) {
		netErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
		err := fmt.Errorf("HTTP request failed: %w", netErr)
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(err))
	})

	t.Run("ErrNotFound is Invalid", func(t *testing.T) {
		require.Equal(t, verifiertypes.TeeSampleInvalid, classifyInfoFetchError(fetcher.ErrNotFound))
	})

	t.Run("ErrRedirect is Invalid", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", fetcher.ErrRedirect)
		require.Equal(t, verifiertypes.TeeSampleInvalid, classifyInfoFetchError(err))
	})

	t.Run("ErrURLValidation is Invalid", func(t *testing.T) {
		err := fmt.Errorf("%w: bad host", verifier.ErrURLValidation)
		require.Equal(t, verifiertypes.TeeSampleInvalid, classifyInfoFetchError(err))
	})

	t.Run("HTTP 500 is Indeterminate", func(t *testing.T) {
		err := &fetcher.HTTPStatusError{URL: "http://x", Code: 500}
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(err))
	})

	t.Run("HTTP 502 is Indeterminate", func(t *testing.T) {
		err := &fetcher.HTTPStatusError{URL: "http://x", Code: 502}
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(err))
	})

	t.Run("HTTP 400 is Invalid", func(t *testing.T) {
		err := &fetcher.HTTPStatusError{URL: "http://x", Code: 400}
		require.Equal(t, verifiertypes.TeeSampleInvalid, classifyInfoFetchError(err))
	})

	t.Run("JSON syntax error is Invalid", func(t *testing.T) {
		var target struct{ X int }
		decodeErr := json.Unmarshal([]byte("not-json"), &target)
		require.Error(t, decodeErr)
		require.Equal(t, verifiertypes.TeeSampleInvalid, classifyInfoFetchError(decodeErr))
	})

	t.Run("JSON type error is Invalid", func(t *testing.T) {
		var target struct{ X int }
		decodeErr := json.Unmarshal([]byte(`{"X":"string"}`), &target)
		require.Error(t, decodeErr)
		require.Equal(t, verifiertypes.TeeSampleInvalid, classifyInfoFetchError(decodeErr))
	})

	t.Run("unknown error is Indeterminate", func(t *testing.T) {
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, classifyInfoFetchError(errors.New("something odd")))
	})
}
