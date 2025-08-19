package verifier

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

func GenerateInstructionId(walletId [32]byte, nonce uint64, sourceEnv string) (common.Hash, error) {
	sourceID, err := utils.Bytes32(sourceEnv)
	if err != nil {
		return common.Hash{}, err
	}
	PAY, err := utils.Bytes32(string(op.Pay))
	if err != nil {
		return common.Hash{}, err
	}
	var nonceByte common.Hash
	nonceBig := big.NewInt(int64(nonce))
	copy(nonceByte[:], common.LeftPadBytes((nonceBig).Bytes(), 32))

	buf := new(bytes.Buffer)
	buf.Write(sourceID[:])
	buf.Write(PAY[:])
	buf.Write(walletId[:])
	buf.Write(nonceByte[:])
	instructionId := crypto.Keccak256Hash(buf.Bytes())
	return instructionId, nil
}

func HexStringToBytes32(s string) (common.Hash, error) {
	var arr common.Hash
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
