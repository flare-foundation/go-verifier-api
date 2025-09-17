package teeinstruction

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
)

func GenerateInstructionID(opType, sourceID [32]byte, senderAddress string, nonce uint64) (common.Hash, error) {
	PAY, err := coreutil.StringToBytes32(string(op.Pay))
	if err != nil {
		return common.Hash{}, err
	}

	args := abi.Arguments{
		{Type: abiType("bytes32")}, // opType
		{Type: abiType("bytes32")}, // PAY
		{Type: abiType("bytes32")}, // sourceId
		{Type: abiType("string")},  // senderAddress
		{Type: abiType("uint64")},  // nonce
	}

	packed, err := args.Pack(opType, PAY, sourceID, senderAddress, nonce)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Keccak256Hash(packed), nil
}

func abiType(t string) abi.Type {
	ty, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(err)
	}
	return ty
}
