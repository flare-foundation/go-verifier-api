package coreutil

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
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
	s = RemoveHexPrefix(s)
	b, err := hex.DecodeString(s)
	if err != nil {
		return arr, err
	}
	if len(b) != Bytes32Size {
		return arr, fmt.Errorf("invalid length for bytes32: got %d bytes, expected %d", len(b), Bytes32Size)
	}
	copy(arr[:], b)
	return arr, nil
}

func StringToABIBytes(s string) []byte {
	strBytes := []byte(s)
	length := len(strBytes)
	buf := new(bytes.Buffer)
	lenBytes := common.LeftPadBytes(big.NewInt(int64(length)).Bytes(), Bytes32Size)
	buf.Write(lenBytes)
	buf.Write(strBytes)
	padding := Bytes32Size - (length % Bytes32Size)
	if padding < Bytes32Size {
		buf.Write(make([]byte, padding))
	}
	return buf.Bytes()
}
