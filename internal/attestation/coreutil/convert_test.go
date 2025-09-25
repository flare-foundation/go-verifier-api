package coreutil

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBytes32(t *testing.T) {
	s := "short"
	b, err := StringToBytes32(s)
	require.NoError(t, err, "unexpected error: %v", err)
	require.Equal(t, s, string(b[:len(s)]), "expected prefix %q, got %q", s, string(b[:len(s)]))

	long := strings.Repeat("a", Bytes32Size+1)
	val, err := StringToBytes32(long)
	var expected [Bytes32Size]byte
	require.Equal(t, expected, val)
	require.ErrorContains(t, err, fmt.Sprintf("string %s too long for Bytes32", long))
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
	tests := []struct {
		name           string
		input          string
		expectedValue  string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:          "valid 32 bytes hex string",
			input:         "0x" + strings.Repeat("a1", 32),
			expectError:   false,
			expectedValue: "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
		},
		{
			name:           "too short hex string",
			input:          "0x1234",
			expectError:    true,
			expectedErrMsg: "invalid length for bytes32: got 2 bytes, expected 32",
		},
		{
			name:           "invalid hex characters",
			input:          "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			expectError:    true,
			expectedErrMsg: "encoding/hex: invalid byte: U+007A 'z'",
		},
		{
			name:           "too long hex string",
			input:          "0x" + strings.Repeat("ff", 33), // 66 hex chars = 33 bytes
			expectError:    true,
			expectedErrMsg: "invalid length for bytes32: got 33 bytes, expected 32",
		},
		{
			name:           "empty string",
			input:          "",
			expectError:    true,
			expectedErrMsg: "invalid length for bytes32: got 0 bytes, expected 32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := HexStringToBytes32(tt.input)
			if tt.expectError {
				require.ErrorContains(t, err, tt.expectedErrMsg)
				require.Equal(t, common.Hash{}, h)
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
				require.Equal(t, tt.expectedValue, hex.EncodeToString(h[:]))
			}
		})
	}
}
