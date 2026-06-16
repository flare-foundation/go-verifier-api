package instruction

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/instructions"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
)

const EventNameTeeInstructionsSent = "TeeInstructionsSent"

func TeeInstructionsSentEventSignature(abiDef abi.ABI) (string, error) {
	event, exists := abiDef.Events[EventNameTeeInstructionsSent]
	if !exists {
		return "", fmt.Errorf("ABI does not contain event %s", EventNameTeeInstructionsSent)
	}
	return event.ID.Hex(), nil
}

// maxEventDataSize caps the byte length of log.Data before ABI decoding.
// Legitimate TeeInstructionsSent events are ~1–2 KB; 1 MB matches the HTTP
// request body limit and prevents OOM from a corrupted indexer row.
const maxEventDataSize = 1 << 20 // 1 MB

// DecodeTeeInstructionsSentEventData decodes a TeeInstructionsSent event into its
// payment instruction message, after binding the event's wrapper op fields back
// to the caller's expectation: the event's OpType must equal expectedOpType and
// its OpCommand must equal command. The instruction-ID topic already commits to
// (opType, op, ...), so a mismatch means the indexed event data disagrees with
// its own topic — a C-chain index inconsistency (db.ErrDatabase, → 503). The
// op fields are not otherwise enforced by decoding into the PAY/REISSUE message
// schema, which is why they are checked explicitly here.
func DecodeTeeInstructionsSentEventData(log *types.Log, teeABI abi.ABI, command op.Command, expectedOpType common.Hash) (*payments.ITeePaymentsPaymentInstructionMessage, error) {
	if len(log.Data) > maxEventDataSize {
		return nil, fmt.Errorf("event data too large (%d bytes, max %d)", len(log.Data), maxEventDataSize)
	}
	eventData, err := abiDecodeEventData[instructions.InstructionsTeeInstructionsSent](
		teeABI,
		EventNameTeeInstructionsSent,
		log.Data,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot decode event %s: %w", EventNameTeeInstructionsSent, err)
	}
	if common.Hash(eventData.OpType) != expectedOpType {
		return nil, fmt.Errorf("DB inconsistency: event OpType %s != expected %s: %w", common.Hash(eventData.OpType).Hex(), expectedOpType.Hex(), db.ErrDatabase)
	}
	expectedOpCommand, err := convert.StringToCommonHash(string(command))
	if err != nil {
		return nil, fmt.Errorf("cannot convert command %q to hash: %w", command, err)
	}
	if common.Hash(eventData.OpCommand) != expectedOpCommand {
		return nil, fmt.Errorf("DB inconsistency: event OpCommand %s != expected %s (%s): %w", common.Hash(eventData.OpCommand).Hex(), expectedOpCommand.Hex(), command, db.ErrDatabase)
	}
	var message payments.ITeePaymentsPaymentInstructionMessage
	err = structs.DecodeTo(payments.MessageArguments[command], eventData.Message, &message)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s message arguments: %w", EventNameTeeInstructionsSent, err)
	}
	return &message, nil
}

func abiDecodeEventData[T any](abiObj abi.ABI, eventName string, data hexutil.Bytes) (*T, error) {
	var result T
	err := abiObj.UnpackIntoInterface(&result, eventName, data)
	if err != nil {
		return nil, fmt.Errorf("ABI unpack into %T failed for event %q: %w", result, eventName, err)
	}
	return &result, nil
}
