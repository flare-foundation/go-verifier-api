package verifier

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const pay = "PAY"

func GenerateInstructionId(walletId [32]byte, nonce uint64, sourceEnv string) string {
	var sourceID [32]byte
	copy(sourceID[:], []byte(sourceEnv))

	var opCommand [32]byte
	copy(opCommand[:], []byte(pay))

	var nonceByte [32]byte
	nonceBig := big.NewInt(int64(nonce))
	copy(nonceByte[:], common.LeftPadBytes((nonceBig).Bytes(), 32))

	instructionId := crypto.Keccak256(sourceID[:], opCommand[:], walletId[:], nonceByte[:])
	return hex.EncodeToString(instructionId)
}

func HexStringToBytes32(s string) ([32]byte, error) {
	var arr [32]byte
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return arr, err
	}
	if len(b) != 32 {
		return arr, fmt.Errorf("invalid length for bytes32: got %d bytes, expected 32", len(b))
	}
	copy(arr[:], b)
	return arr, nil
}

func NewBigIntFromString(s string) (*big.Int, error) {
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid big.Int string: %s", s)
	}
	return i, nil
}

func GetStringField(m map[string]interface{}, key string) (string, bool) {
	val, ok := m[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func GetStandardAddressHash(address string) string {
	hash := crypto.Keccak256([]byte(address))
	return fmt.Sprintf("0x%x", hash)
}
