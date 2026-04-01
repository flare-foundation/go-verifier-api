package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTeePollerSampleState_String(t *testing.T) {
	tests := []struct {
		state    TeeSampleState
		expected string
	}{
		{TeeSampleValid, "VALID"},
		{TeeSampleInvalid, "INVALID"},
		{TeeSampleIndeterminate, "INDETERMINATE"},
		{TeeSampleState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.state.String())
	}
}

func TestTeePollerSampleState_MarshalJSON(t *testing.T) {
	tests := []struct {
		state    TeeSampleState
		expected string
	}{
		{TeeSampleValid, `"VALID"`},
		{TeeSampleInvalid, `"INVALID"`},
		{TeeSampleIndeterminate, `"INDETERMINATE"`},
		{TeeSampleState(99), `"UNKNOWN"`},
	}

	for _, tt := range tests {
		b, err := json.Marshal(tt.state)
		require.NoError(t, err)
		require.Equal(t, tt.expected, string(b))
	}
}

func TestTeePollerSample_StructJSON(t *testing.T) {
	sample := TeeSampleValue{
		Timestamp: time.Date(2025, 11, 10, 0, 0, 0, 0, time.UTC),
		State:     TeeSampleValid,
	}

	b, err := json.Marshal(sample)
	require.NoError(t, err)
	require.Contains(t, string(b), `"VALID"`)
}
