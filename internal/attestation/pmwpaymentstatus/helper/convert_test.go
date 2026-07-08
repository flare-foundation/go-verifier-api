package helper

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExceedsMaxXRPDrops(t *testing.T) {
	t.Run("nil fails closed", func(t *testing.T) {
		require.True(t, ExceedsMaxXRPDrops(nil))
	})
	t.Run("within supply", func(t *testing.T) {
		require.False(t, ExceedsMaxXRPDrops(big.NewInt(1)))
		require.False(t, ExceedsMaxXRPDrops(new(big.Int).SetUint64(MaxXRPDrops)))
	})
	t.Run("above supply", func(t *testing.T) {
		require.True(t, ExceedsMaxXRPDrops(new(big.Int).SetUint64(MaxXRPDrops+1)))
	})
	t.Run("exceeds uint64", func(t *testing.T) {
		huge := new(big.Int).Lsh(big.NewInt(1), 65) // 2^65, not representable as uint64
		require.True(t, ExceedsMaxXRPDrops(huge))
	})
}

func TestParseNonNegativeBigInt(t *testing.T) {
	t.Run("valid number", func(t *testing.T) {
		input := "1234567890"
		val, err := ParseNonNegativeBigInt(input)
		require.NoError(t, err)
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading zeros", func(t *testing.T) {
		input := "00001234"
		val, err := ParseNonNegativeBigInt(input)
		require.NoError(t, err)
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading and trailing whitespace", func(t *testing.T) {
		input := "   1234  "
		val, err := ParseNonNegativeBigInt(strings.TrimSpace(input))
		require.NoError(t, err)
		expected := big.NewInt(1234)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("value at the digit cap accepted", func(t *testing.T) {
		input := strings.Repeat("9", maxDropsDigits)
		val, err := ParseNonNegativeBigInt(input)
		require.NoError(t, err)
		require.Equal(t, input, val.String())
	})
	t.Run("overlong fee-like string rejected before parse", func(t *testing.T) {
		// A multi-KB decimal Fee from a corrupt indexer must be rejected on length,
		// not parsed (DOS-2 guards both the Fee and Balance call sites, which both
		// route through ParseNonNegativeBigInt).
		input := strings.Repeat("9", 5000)
		val, err := ParseNonNegativeBigInt(input)
		require.ErrorContains(t, err, "decimal value too long")
		require.Nil(t, val)
	})
	t.Run("overlong balance-like string rejected before parse", func(t *testing.T) {
		input := strings.Repeat("1", maxDropsDigits+1)
		val, err := ParseNonNegativeBigInt(input)
		require.ErrorContains(t, err, "decimal value too long")
		require.Nil(t, val)
	})
	t.Run("invalid input", func(t *testing.T) {
		input := "notanumber"
		val, err := ParseNonNegativeBigInt(input)
		require.ErrorContains(t, err, "invalid big.Int string: notanumber")
		require.Nil(t, val)
	})
	t.Run("negative value rejected", func(t *testing.T) {
		val, err := ParseNonNegativeBigInt("-1")
		require.ErrorContains(t, err, "expected non-negative big.Int, got -1")
		require.Nil(t, val)
	})
	t.Run("zero accepted", func(t *testing.T) {
		val, err := ParseNonNegativeBigInt("0")
		require.NoError(t, err)
		require.Equal(t, big.NewInt(0).String(), val.String())
	})
	t.Run("large negative rejected", func(t *testing.T) {
		// Within the length cap so this exercises the sign check, not the cap.
		input := "-" + strings.Repeat("9", 20)
		val, err := ParseNonNegativeBigInt(input)
		require.ErrorContains(t, err, "expected non-negative big.Int")
		require.Nil(t, val)
	})
}
