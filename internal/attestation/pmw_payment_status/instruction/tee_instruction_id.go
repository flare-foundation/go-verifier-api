package teeinstruction

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
)

func GenerateInstructionID(opType, sourceID [32]byte, senderAddress string, nonce uint64) (common.Hash, error) {
	PAY, err := coreutil.StringToBytes32(string(op.Pay)) // TODO Do we really need this error? Since op.Pay is fixed ...
	if err != nil {
		return common.Hash{}, err
	}
	senderAddressByte := coreutil.StringToABIBytes(senderAddress)
	nonceByte := common.LeftPadBytes((big.NewInt(int64(nonce))).Bytes(), coreutil.Bytes32Size)

	buf := new(bytes.Buffer)
	buf.Write(opType[:])
	buf.Write(PAY[:])
	buf.Write(sourceID[:])
	buf.Write(senderAddressByte)
	buf.Write(nonceByte)
	instructionID := crypto.Keccak256Hash(buf.Bytes())
	return instructionID, nil
}
