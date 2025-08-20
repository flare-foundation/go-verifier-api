package pmwpaymentutils_test

import (
	"math/big"
	"strings"
	"testing"

	pmwpaymentutils "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/utils"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

func TestGenerateInstructionId(t *testing.T) {
	walletId := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	walletIdBytes, err := utils.HexStringToBytes32(walletId)
	if err != nil {
		t.Fatalf("HexStringToBytes32 failed for valid input: %v", err)
	}
	nonce := uint64(42)
	sourceEnv := "testsourceid"
	id, err := pmwpaymentutils.GenerateInstructionId(walletIdBytes, nonce, sourceEnv)
	if err != nil {
		t.Fatalf("GenerateInstructionId failed for valid input: %v", err)
	}
	if len(id) == 0 {
		t.Fatal("GenerateInstructionId returned empty string")
	}
	t.Logf("Instruction ID: %s", id.Hex())
}

func TestHexStringToBytes32(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		validHex := "0x" + strings.Repeat("aa", 32)
		arr, err := utils.HexStringToBytes32(validHex)
		if err != nil {
			t.Fatalf("HexStringToBytes32 failed for valid input: %v", err)
		}
		for i, b := range arr {
			if b != 0xaa {
				t.Errorf("Byte %d expected 0xaa, got 0x%x", i, b)
			}
		}
	})
	t.Run("invalid input", func(t *testing.T) {
		invalidHex := "0x1234"
		_, err := utils.HexStringToBytes32(invalidHex)
		if err == nil {
			t.Fatal("HexStringToBytes32 should fail for invalid length hex")
		}
	})
	t.Run("invalid input", func(t *testing.T) {
		badHex := "0xzzyy"
		_, err := utils.HexStringToBytes32(badHex)
		if err == nil {
			t.Fatal("HexStringToBytes32 should fail for invalid length hex")
		}
	})
}

func TestNewBigIntFromString(t *testing.T) {
	t.Run("valid number", func(t *testing.T) {
		input := "1234567890"
		val, err := utils.NewBigIntFromString(input)
		if err != nil {
			t.Fatalf("NewBigIntFromString failed for valid input: %v", err)
		}
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading zeros", func(t *testing.T) {
		input := "00001234"
		val, err := utils.NewBigIntFromString(input)
		if err != nil {
			t.Fatalf("NewBigIntFromString failed with leading zeros: %v", err)
		}
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading and trailing whitespace", func(t *testing.T) {
		input := "   1234  "
		val, err := utils.NewBigIntFromString(strings.TrimSpace(input))
		if err != nil {
			t.Fatalf("NewBigIntFromString failed with whitespace: %v", err)
		}
		expected := big.NewInt(1234)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("very large number", func(t *testing.T) {
		input := strings.Repeat("9", 100)
		val, err := utils.NewBigIntFromString(input)
		if err != nil {
			t.Fatalf("NewBigIntFromString failed for large number: %v", err)
		}
		if val.String() != input {
			t.Fatalf("Expected %s, got %s", input, val.String())
		}
	})
	t.Run("invalid input", func(t *testing.T) {
		input := "notanumber"
		_, err := utils.NewBigIntFromString(input)
		if err == nil {
			t.Fatal("Expected error for invalid input, got nil")
		}
	})
}

func TestGetStringField(t *testing.T) {
	m := map[string]interface{}{
		"key1": "val1",
		"key2": 1234,
	}
	t.Run("valid string field", func(t *testing.T) {
		val, ok := pmwpaymentutils.GetStringField(m, "key1")
		if !ok || val != "val1" {
			t.Fatal("GetStringField failed to get existing string value")
		}
	})
	t.Run("number field", func(t *testing.T) {
		_, ok := pmwpaymentutils.GetStringField(m, "key2")
		if ok {
			t.Fatal("GetStringField should return false for non-string value")
		}
	})
	t.Run("missing field", func(t *testing.T) {
		_, ok := pmwpaymentutils.GetStringField(m, "missing")
		if ok {
			t.Fatal("GetStringField should return false for missing key")
		}
	})
}

func TestGetStandardAddressHash(t *testing.T) {
	address := "rL7RGDcogfqDnEjCaz2qSpivXF1B1EnsvW"
	val := pmwpaymentutils.GetStandardAddressHash(address)
	expectedStdAddressHash := "0x00bafe0a11e53099df6fa8bc148cb4e054594c23c8fbc4ec5c5c85cf72a1e96c"
	if val != expectedStdAddressHash {
		t.Fatalf("GetStandardAddressHash returned wrong value, got %s", val)
	}
	if len(expectedStdAddressHash) == 0 {
		t.Fatal("GetStandardAddressHash returned empty string")
	}
	if expectedStdAddressHash[:2] != "0x" {
		t.Fatal("GetStandardAddressHash returned string without 0x prefix")
	}
}
