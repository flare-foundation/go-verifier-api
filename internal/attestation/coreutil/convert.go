package coreutil

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

const (
	Bytes32Size = 32
)

func StringToBytes32(s string) ([32]byte, error) {
	var b [32]byte
	if len(s) > Bytes32Size {
		return b, fmt.Errorf("string %s too long for Bytes32", s)
	}
	copy(b[:], s)
	return b, nil
}

func RemoveHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

func HexStringToBytes32(s string) (common.Hash, error) {
	var arr common.Hash
	st := RemoveHexPrefix(s)
	b, err := hex.DecodeString(st)
	if err != nil {
		return arr, fmt.Errorf("invalid hex string %s: %w", s, err)
	}
	if len(b) != Bytes32Size {
		return arr, fmt.Errorf("invalid length for bytes32 hex string: got %d bytes, want %d (%s)", len(b), Bytes32Size, s)
	}
	copy(arr[:], b)
	return arr, nil
}
