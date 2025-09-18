package instruction

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
)

func GenerateChallengeInstructionID(teeID common.Address, challenge common.Hash) (common.Hash, error) {
	REG_OP_TYPE, err := coreutil.StringToBytes32(string(op.Reg))
	if err != nil {
		return common.Hash{}, err
	}
	TEE_ATTESTATION, err := coreutil.StringToBytes32(string(op.TEEAttestation))
	if err != nil {
		return common.Hash{}, err
	}
	args := abi.Arguments{
		{Type: coreutil.AbiType("bytes32")}, // REG_OP_TYPE
		{Type: coreutil.AbiType("bytes32")}, // TEE_ATTESTATION
		{Type: coreutil.AbiType("address")}, // teeID
		{Type: coreutil.AbiType("bytes32")}, // challenge
	}
	packed, err := args.Pack(REG_OP_TYPE, TEE_ATTESTATION, teeID, challenge)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Keccak256Hash(packed), nil
}
