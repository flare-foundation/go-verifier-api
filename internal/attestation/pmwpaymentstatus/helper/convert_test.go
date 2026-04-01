package helper

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
		val, err := ParseBigInt(input)
		require.ErrorContains(t, err, "invalid big.Int string: notanumber")
		require.Nil(t, val)
	})
}
