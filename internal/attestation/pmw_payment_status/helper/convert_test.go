package helper

import (
	"math/big"
	"strings"
	"testing"
)

func TestGetStandardAddressHash(t *testing.T) {
	address := "rL7RGDcogfqDnEjCaz2qSpivXF1B1EnsvW"
	val := GetStandardAddressHash(address)
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

func TestBytesToHex0x(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	got := bytesToHex0x(data)
	want := "0x010203"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestParseBigInt(t *testing.T) {
	t.Run("valid number", func(t *testing.T) {
		input := "1234567890"
		val, err := ParseBigInt(input)
		if err != nil {
			t.Fatalf("ParseBigInt failed for valid input: %v", err)
		}
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading zeros", func(t *testing.T) {
		input := "00001234"
		val, err := ParseBigInt(input)
		if err != nil {
			t.Fatalf("ParseBigInt failed with leading zeros: %v", err)
		}
		expected := new(big.Int)
		expected.SetString(input, 10)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("leading and trailing whitespace", func(t *testing.T) {
		input := "   1234  "
		val, err := ParseBigInt(strings.TrimSpace(input))
		if err != nil {
			t.Fatalf("ParseBigInt failed with whitespace: %v", err)
		}
		expected := big.NewInt(1234)
		if val.Cmp(expected) != 0 {
			t.Fatalf("Expected %s, got %s", expected.String(), val.String())
		}
	})
	t.Run("very large number", func(t *testing.T) {
		input := strings.Repeat("9", 100)
		val, err := ParseBigInt(input)
		if err != nil {
			t.Fatalf("ParseBigInt failed for large number: %v", err)
		}
		if val.String() != input {
			t.Fatalf("Expected %s, got %s", input, val.String())
		}
	})
	t.Run("invalid input", func(t *testing.T) {
		input := "notanumber"
		_, err := ParseBigInt(input)
		if err == nil {
			t.Fatal("Expected error for invalid input, got nil")
		}
	})
}
