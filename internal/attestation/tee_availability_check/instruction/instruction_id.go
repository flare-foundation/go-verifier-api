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
		{Type: coreutil.Bytes32Type}, // REG_OP_TYPE
		{Type: coreutil.Bytes32Type}, // TEE_ATTESTATION
		{Type: coreutil.AddressType}, // teeID
		{Type: coreutil.Bytes32Type}, // challenge
	}
	packed, err := args.Pack(REG_OP_TYPE, TEE_ATTESTATION, teeID, challenge)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Keccak256Hash(packed), nil
}
