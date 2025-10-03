package instruction

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
)

func GenerateChallengeInstructionID(teeID common.Address, challenge common.Hash) (common.Hash, error) {
	REG_OP_TYPE, err := coreutil.StringToBytes32(string(op.Reg))
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot convert REG_OP_TYPE to Bytes32: %w", err) // Should never happen.
	}
	TEE_ATTESTATION, err := coreutil.StringToBytes32(string(op.TEEAttestation))
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot convert TEE_ATTESTATION to Bytes32: %w", err) // Should never happen.
	}
	args := abi.Arguments{
		{Type: coreutil.Bytes32Type}, // REG_OP_TYPE
		{Type: coreutil.Bytes32Type}, // TEE_ATTESTATION
		{Type: coreutil.AddressType}, // teeID
		{Type: coreutil.Bytes32Type}, // challenge
	}
	packed, err := args.Pack(REG_OP_TYPE, TEE_ATTESTATION, teeID, challenge)
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot pack ABI arguments: %w", err)
	}
	return crypto.Keccak256Hash(packed), nil
}
