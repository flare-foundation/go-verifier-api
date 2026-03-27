package instruction

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/helper"
	paymentinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/instruction"
)

// GeneratePayInstructionID delegates to PMWPaymentStatus's GenerateInstructionID — same encoding.
var GeneratePayInstructionID = paymentinstruction.GenerateInstructionID

func GenerateReissueInstructionID(opType, sourceID [32]byte, senderAddress string, nonce uint64, reissueNumber uint64) (common.Hash, error) {
	reissue, err := convert.StringToCommonHash(string(op.Reissue))
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot convert REISSUE to Bytes32: %w", err)
	}
	args := abi.Arguments{
		{Type: helper.Bytes32Type}, // opType
		{Type: helper.Bytes32Type}, // REISSUE
		{Type: helper.Bytes32Type}, // sourceId
		{Type: helper.StringType},  // senderAddress
		{Type: helper.Uint64Type},  // nonce
		{Type: helper.Uint64Type},  // reissueNumber
	}
	packed, err := args.Pack(opType, reissue, sourceID, senderAddress, nonce, reissueNumber)
	if err != nil {
		return common.Hash{}, fmt.Errorf("cannot pack ABI arguments: %w", err)
	}
	return crypto.Keccak256Hash(packed), nil
}
