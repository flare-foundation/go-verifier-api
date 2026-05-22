package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTeeSampleStateString(t *testing.T) {
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

func TestTeeSampleStateMarshalJSON(t *testing.T) {
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
