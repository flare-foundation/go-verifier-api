package teepoller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/fetcher"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
)

// classifyInfoFetchError maps errors from fetchTEEInfoData (URL resolution +
// HTTP fetch + JSON decode) to a TeeSampleState.
//
// INVALID is reserved for deterministic faults attributable to the TEE or its
// registry entry (endpoint missing, bad URL, rejected redirect, malformed
// response body). INDETERMINATE covers transient transport/verifier-side
// conditions that should not contribute to a DOWN classification on their own.
//
// See L-11 in audit.md for the rationale.
func classifyInfoFetchError(err error) verifiertypes.TeeSampleState {
	if err == nil {
		return verifiertypes.TeeSampleValid
	}
	// Shared transport layer (context, net.Error) → INDETERMINATE.
	if state, _, ok := verifiertypes.MapTransportError(err); ok {
		return state
	}
	// Deterministic TEE-side faults.
	switch {
	case errors.Is(err, fetcher.ErrNotFound):
		return verifiertypes.TeeSampleInvalid
	case errors.Is(err, fetcher.ErrRedirect):
		return verifiertypes.TeeSampleInvalid
	case errors.Is(err, verifier.ErrURLValidation):
		return verifiertypes.TeeSampleInvalid
	}
	// HTTP non-2xx: classify with TEE-endpoint semantics. 4xx are
	// deterministic endpoint faults (TEE proxy misconfiguration), except
	// 408 (request timeout) and 429 (rate limit) which are transient.
	// 5xx are transient server/proxy issues. This differs from the RPC
	// path where all non-400/404 are INDETERMINATE (verifier infra fault).
	var statusErr *fetcher.HTTPStatusError
	if errors.As(err, &statusErr) {
		code := statusErr.Code
		if code >= 400 && code < 500 &&
			code != http.StatusRequestTimeout &&
			code != http.StatusTooManyRequests {
			return verifiertypes.TeeSampleInvalid
		}
		return verifiertypes.TeeSampleIndeterminate
	}
	// Malformed or truncated JSON from the TEE → deterministic TEE fault.
	var jsonSyntaxErr *json.SyntaxError
	var jsonTypeErr *json.UnmarshalTypeError
	if errors.As(err, &jsonSyntaxErr) || errors.As(err, &jsonTypeErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return verifiertypes.TeeSampleInvalid
	}
	// Unknown error — fail safe as INDETERMINATE (avoid false DOWN).
	return verifiertypes.TeeSampleIndeterminate
}
