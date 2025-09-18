package instruction

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
		{Type: coreutil.AbiType("bytes32")}, // opType
		{Type: coreutil.AbiType("bytes32")}, // PAY
		{Type: coreutil.AbiType("bytes32")}, // sourceId
		{Type: coreutil.AbiType("string")},  // senderAddress
		{Type: coreutil.AbiType("uint64")},  // nonce
	}
	packed, err := args.Pack(opType, PAY, sourceID, senderAddress, nonce)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Keccak256Hash(packed), nil
}
