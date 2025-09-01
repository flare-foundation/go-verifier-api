package utils

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/test_util"

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
	want := "0x010203"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
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

	tests := []test_util.TestCase[string, string]{
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
