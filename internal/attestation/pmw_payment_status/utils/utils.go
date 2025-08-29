package pmwpaymentutils

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

func GenerateInstructionId(walletId, opType [32]byte, nonce uint64) (common.Hash, error) {
	PAY, err := utils.Bytes32(string(op.Pay))
	if err != nil {
		return common.Hash{}, err
	}
	var nonceByte common.Hash
	nonceBig := big.NewInt(int64(nonce))
	copy(nonceByte[:], common.LeftPadBytes((nonceBig).Bytes(), utils.Bytes32Size))

	buf := new(bytes.Buffer)
	buf.Write(opType[:])
	buf.Write(PAY[:])
	buf.Write(walletId[:])
	buf.Write(nonceByte[:])
	instructionId := crypto.Keccak256Hash(buf.Bytes())
	return instructionId, nil
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
	return utils.BytesToHex0x(hash)
}
