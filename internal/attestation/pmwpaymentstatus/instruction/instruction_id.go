package instruction

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/helper"
)

func GenerateInstructionID(opType, sourceID [32]byte, accountAddress string, paymentId uint64) (common.Hash, error) {
	PAY, err := convert.StringToCommonHash(string(op.Pay))
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot convert PAY to Bytes32: %w", err)
	}
	// instructionId = keccak(opType, PAY, sourceId, accountAddress, paymentId, reissueNumber);
	// PAY events always use reissueNumber 0 (reissues carry a non-zero number).
	args := abi.Arguments{
		{Type: helper.Bytes32Type}, // opType
		{Type: helper.Bytes32Type}, // PAY
		{Type: helper.Bytes32Type}, // sourceId
		{Type: helper.StringType},  // accountAddress
		{Type: helper.Uint64Type},  // paymentId
		{Type: helper.Uint64Type},  // reissueNumber (0 for PAY)
	}
	packed, err := args.Pack(opType, PAY, sourceID, accountAddress, paymentId, uint64(0))
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot pack ABI arguments: %w", err)
	}
	return crypto.Keccak256Hash(packed), nil
}
