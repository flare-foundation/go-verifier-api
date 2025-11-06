package instruction_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/instruction"
	"github.com/stretchr/testify/require"
)

func TestGenerateInstructionID(t *testing.T) {
	expected := "0x99dac3c061c7beb1d87cf2dc5beb9f0a6df256f03cc5a7213247778b51e7291d"
	senderAddress := "r9CWG1aj4tUsZn5agTLahfyiqnNhMhPjDt"
	nonce := uint64(10702286)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash("XRP")
	require.NoError(t, err)
	id, err := instruction.GenerateInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Equal(t, expected, id.Hex())
}
