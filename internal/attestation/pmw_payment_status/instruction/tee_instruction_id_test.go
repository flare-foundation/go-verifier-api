package teeinstruction_test

import (
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/instruction"
	"github.com/stretchr/testify/require"
)

func TestGenerateInstructionId(t *testing.T) {
	senderAddress := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	nonce := uint64(42)
	opTypeString := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	opTypeBytes, err := coreutil.HexStringToBytes32(opTypeString)
	require.NoError(t, err)
	sourceIDBytes := opTypeBytes
	id, err := teeinstruction.GenerateInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	t.Logf("Instruction ID: %s", id.Hex())
}
