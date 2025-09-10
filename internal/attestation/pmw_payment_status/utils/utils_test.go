package pmwpaymentutils_test

import (
	"math/big"
	"strings"
	"testing"

	pmwpaymentutils "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/utils"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/stretchr/testify/require"
)

func TestGenerateInstructionId(t *testing.T) {
	walletId := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	walletIdBytes, err := utils.HexStringToBytes32(walletId)
	require.NoError(t, err)
	nonce := uint64(42)
	opTypeString := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	opTypeBytes, err := utils.HexStringToBytes32(opTypeString)
	require.NoError(t, err)
	id, err := pmwpaymentutils.GenerateInstructionId(walletIdBytes, opTypeBytes, nonce)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	t.Logf("Instruction ID: %s", id.Hex())
}

func TestHexStringToBytes32(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		validHex := "0x" + strings.Repeat("aa", 32)
		arr, err := utils.HexStringToBytes32(validHex)
		require.NoError(t, err)
		for _, b := range arr {
			require.Equal(t, byte(0xaa), b)
		}
	})
	t.Run("invalid input", func(t *testing.T) {
		invalidHex := "0x1234"
		_, err := utils.HexStringToBytes32(invalidHex)
		require.Error(t, err)
	})
	t.Run("invalid input", func(t *testing.T) {
		badHex := "0xzzyy"
		_, err := utils.HexStringToBytes32(badHex)
		require.Error(t, err)
	})
}

func TestNewBigIntFromString(t *testing.T) {
	t.Run("valid number", func(t *testing.T) {
		input := "1234567890"
		val, err := utils.NewBigIntFromString(input)
		require.NoError(t, err)
		expected := new(big.Int)
		expected.SetString(input, 10)
		require.Equal(t, expected, val)
	})
	t.Run("leading zeros", func(t *testing.T) {
		input := "00001234"
		val, err := utils.NewBigIntFromString(input)
		require.NoError(t, err)
		expected := new(big.Int)
		expected.SetString(input, 10)
		require.Equal(t, expected, val)
	})
	t.Run("leading and trailing whitespace", func(t *testing.T) {
		input := "   1234  "
		val, err := utils.NewBigIntFromString(strings.TrimSpace(input))
		require.NoError(t, err)
		expected := big.NewInt(1234)
		require.Equal(t, expected, val)
	})
	t.Run("very large number", func(t *testing.T) {
		input := strings.Repeat("9", 100)
		val, err := utils.NewBigIntFromString(input)
		require.NoError(t, err)
		require.Equal(t, input, val.String())
	})
	t.Run("invalid input", func(t *testing.T) {
		input := "notanumber"
		_, err := utils.NewBigIntFromString(input)
		require.Error(t, err)
	})
}

func TestGetStringField(t *testing.T) {
	m := map[string]interface{}{
		"key1": "val1",
		"key2": 1234,
	}
	t.Run("valid string field", func(t *testing.T) {
		val, ok := pmwpaymentutils.GetStringField(m, "key1")
		require.True(t, ok)
		require.Equal(t, "val1", val)
	})
	t.Run("number field", func(t *testing.T) {
		_, ok := pmwpaymentutils.GetStringField(m, "key2")
		require.False(t, ok)
	})
	t.Run("missing field", func(t *testing.T) {
		_, ok := pmwpaymentutils.GetStringField(m, "missing")
		require.False(t, ok)
	})
}

func TestGetStandardAddressHash(t *testing.T) {
	address := "rL7RGDcogfqDnEjCaz2qSpivXF1B1EnsvW"
	val := pmwpaymentutils.GetStandardAddressHash(address)
	expectedStdAddressHash := "0x00bafe0a11e53099df6fa8bc148cb4e054594c23c8fbc4ec5c5c85cf72a1e96c"
	require.Equal(t, expectedStdAddressHash, val)
	require.NotEmpty(t, expectedStdAddressHash)
	require.Equal(t, "0x", expectedStdAddressHash[:2])
}
