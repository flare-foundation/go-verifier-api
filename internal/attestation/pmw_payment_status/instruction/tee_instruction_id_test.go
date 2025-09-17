package teeinstruction_test

import (
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/instruction"
	"github.com/stretchr/testify/require"
)

func TestGenerateInstructionID(t *testing.T) {
	senderAddress := "r9CWG1aj4tUsZn5agTLahfyiqnNhMhPjDt"
	nonce := uint64(10702286)
	opTypeBytes, err := coreutil.StringToBytes32(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := coreutil.StringToBytes32("XRP")
	require.NoError(t, err)
	id, err := teeinstruction.GenerateInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	t.Logf("Instruction ID: %s", id.Hex())
	require.Equal(t, "0x99dac3c061c7beb1d87cf2dc5beb9f0a6df256f03cc5a7213247778b51e7291d", id.Hex())
}
