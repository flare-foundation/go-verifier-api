package teetypes_test

import (
	"encoding/json"
	"testing"
	"time"

	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/stretchr/testify/require"
)

func TestTeePollerSampleState_String(t *testing.T) {
	tests := []struct {
		state    teetypes.TeePollerSampleState
		expected string
	}{
		{teetypes.TeePollerSampleValid, "VALID"},
		{teetypes.TeePollerSampleInvalid, "INVALID"},
		{teetypes.TeePollerSampleIndeterminate, "INDETERMINATE"},
		{teetypes.TeePollerSampleState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.state.String())
	}
}

func TestTeePollerSampleState_MarshalJSON(t *testing.T) {
	tests := []struct {
		state    teetypes.TeePollerSampleState
		expected string
	}{
		{teetypes.TeePollerSampleValid, `"VALID"`},
		{teetypes.TeePollerSampleInvalid, `"INVALID"`},
		{teetypes.TeePollerSampleIndeterminate, `"INDETERMINATE"`},
		{teetypes.TeePollerSampleState(99), `"UNKNOWN"`},
	}

	for _, tt := range tests {
		b, err := json.Marshal(tt.state)
		require.NoError(t, err)
		require.Equal(t, tt.expected, string(b))
	}
}

func TestTeePollerSample_StructJSON(t *testing.T) {
	sample := teetypes.TeePollerSample{
		Timestamp: time.Date(2025, 11, 10, 0, 0, 0, 0, time.UTC),
		State:     teetypes.TeePollerSampleValid,
	}

	b, err := json.Marshal(sample)
	require.NoError(t, err)
	require.Contains(t, string(b), `"VALID"`)
}
