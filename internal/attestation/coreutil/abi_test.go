package coreutil

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestMustAbiType_ValidTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected abi.Type
	}{
		{"bytes32", Bytes32Type},
		{"address", AddressType},
		{"uint64", Uint64Type},
		{"string", StringType},
	}

	for _, tt := range tests {
		ty := mustAbiType(tt.input)
		require.Equal(t, tt.expected, ty)
	}
}

func TestMustAbiType_PanicOnInvalidType(t *testing.T) {
	require.PanicsWithValue(t, "invalid ABI type: unsupported arg type: invalid", func() {
		mustAbiType("invalid")
	})
}
