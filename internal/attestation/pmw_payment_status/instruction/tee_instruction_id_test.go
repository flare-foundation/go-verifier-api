package teeinstruction_test

import (
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/instruction"
	"github.com/stretchr/testify/require"
)

func TestGenerateInstructionId(t *testing.T) {
	walletId := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	walletIdBytes, err := coreutil.HexStringToBytes32(walletId)
	require.NoError(t, err)
	nonce := uint64(42)
	opTypeString := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	opTypeBytes, err := coreutil.HexStringToBytes32(opTypeString)
	require.NoError(t, err)
	id, err := teeinstruction.GenerateInstructionID(walletIdBytes, opTypeBytes, nonce)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	t.Logf("Instruction ID: %s", id.Hex())
}
