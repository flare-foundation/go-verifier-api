package teeinstruction

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	utils "github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
)

func GenerateInstructionID(walletID, opType [32]byte, nonce uint64) (common.Hash, error) {
	PAY, err := utils.StringToBytes32(string(op.Pay))
	if err != nil {
		return common.Hash{}, err
	}
	var nonceByte common.Hash
	nonceBig := big.NewInt(int64(nonce))
	copy(nonceByte[:], common.LeftPadBytes((nonceBig).Bytes(), utils.Bytes32Size))

	buf := new(bytes.Buffer)
	buf.Write(opType[:])
	buf.Write(PAY[:])
	buf.Write(walletID[:])
	buf.Write(nonceByte[:])
	instructionID := crypto.Keccak256Hash(buf.Bytes())
	return instructionID, nil
}
