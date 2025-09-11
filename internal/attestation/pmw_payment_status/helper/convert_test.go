package helper

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetStandardAddressHash(t *testing.T) {
	address := "rL7RGDcogfqDnEjCaz2qSpivXF1B1EnsvW"
	val := GetStandardAddressHash(address)
	expectedStdAddressHash := "0x00bafe0a11e53099df6fa8bc148cb4e054594c23c8fbc4ec5c5c85cf72a1e96c"
	require.Equal(t, expectedStdAddressHash, val)
	require.NotEmpty(t, expectedStdAddressHash)
	require.Equal(t, "0x", expectedStdAddressHash[:2])

}

func TestBytesToHex0x(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	got := bytesToHex0x(data)
	want := "0x010203"
	require.Equal(t, want, got)
}

func TestParseBigInt(t *testing.T) {
	t.Run("valid number", func(t *testing.T) {
		input := "1234567890"
		val, err := ParseBigInt(input)
		require.NoError(t, err)
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading zeros", func(t *testing.T) {
		input := "00001234"
		val, err := ParseBigInt(input)
		require.NoError(t, err)
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading and trailing whitespace", func(t *testing.T) {
		input := "   1234  "
		val, err := ParseBigInt(strings.TrimSpace(input))
		require.NoError(t, err)
		expected := big.NewInt(1234)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("very large number", func(t *testing.T) {
		input := strings.Repeat("9", 100)
		val, err := ParseBigInt(input)
		require.NoError(t, err)
		require.Equal(t, input, val.String())
	})
	t.Run("invalid input", func(t *testing.T) {
		input := "notanumber"
		_, err := ParseBigInt(input)
		require.Error(t, err)
	})
}
