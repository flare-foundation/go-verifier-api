package helper

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestMustAbiType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    abi.Type
		expectPanic bool
		panicMsg    string
	}{
		{"bytes32 type", "bytes32", Bytes32Type, false, ""},
		{"uint64 type", "uint64", Uint64Type, false, ""},
		{"string type", "string", StringType, false, ""},
		{"invalid type", "invalid", abi.Type{}, true, "invalid ABI type: unsupported arg type: invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				require.PanicsWithValue(t, tt.panicMsg, func() {
					mustAbiType(tt.input)
				})
			} else {
				got := mustAbiType(tt.input)
				require.Equal(t, tt.expected, got)
			}
		})
	}
}
