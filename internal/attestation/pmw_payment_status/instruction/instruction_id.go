package instruction

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
)

func GenerateInstructionID(opType, sourceID [32]byte, senderAddress string, nonce uint64) (common.Hash, error) {
	PAY, err := convert.StringToCommonHash(string(op.Pay))
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot convert PAY to Bytes32: %w", err) // Should never happen.
	}
	args := abi.Arguments{
		{Type: coreutil.Bytes32Type}, // opType
		{Type: coreutil.Bytes32Type}, // PAY
		{Type: coreutil.Bytes32Type}, // sourceId
		{Type: coreutil.StringType},  // senderAddress
		{Type: coreutil.Uint64Type},  // nonce
	}
	packed, err := args.Pack(opType, PAY, sourceID, senderAddress, nonce)
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot pack ABI arguments: %w", err)
	}
	return crypto.Keccak256Hash(packed), nil
}
