package helper

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
)

func ParseBigInt(s string) (*big.Int, error) {
	const decimalBase = 10
	i, ok := new(big.Int).SetString(s, decimalBase)
	if !ok {
		return nil, fmt.Errorf("invalid big.Int string: %s", s)
	}
	return i, nil
}

func GetStandardAddressHash(address string) string {
	hash := crypto.Keccak256([]byte(address))
	return bytesToHex0x(hash)
}

func bytesToHex0x(data []byte) string {
	return "0x" + hex.EncodeToString(data)
}
