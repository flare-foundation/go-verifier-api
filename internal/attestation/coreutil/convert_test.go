package coreutil

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestStringToBytes32(t *testing.T) {
	t.Run("valid short string", func(t *testing.T) {
		s := "short"
		b, err := StringToBytes32(s)
		require.NoError(t, err)
		require.Equal(t, s, string(b[:len(s)]), "expected prefix %q, got %q", s, string(b[:len(s)]))
	})
	t.Run("too long string", func(t *testing.T) {
		long := strings.Repeat("a", Bytes32Size+1)
		val, err := StringToBytes32(long)
		var expected [Bytes32Size]byte
		require.Equal(t, expected, val)
		require.ErrorContains(t, err, "too long for Bytes32")
	})
}

func TestRemoveHexPrefix(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"lowercase 0x", "0xabc", "abc"},
		{"uppercase 0X", "0Xabc", "abc"},
		{"no prefix", "abc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveHexPrefix(tt.in)
			require.Equal(t, tt.want, got, "RemoveHexPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		})
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
			expectedErrMsg: "invalid length for bytes32 hex string: got 2 bytes, want 32 (0x1234)",
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
			expectedErrMsg: "invalid length for bytes32 hex string: got 33 bytes, want 32 (0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff)",
		},
		{
			name:           "empty string",
			input:          "",
			expectError:    true,
			expectedErrMsg: "invalid length for bytes32 hex string: got 0 bytes, want 32 ()",
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
