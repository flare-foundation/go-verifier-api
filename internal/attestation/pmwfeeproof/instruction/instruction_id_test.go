package instruction_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGeneratePayInstructionID(t *testing.T) {
	// Same expected value as PMWPaymentStatus GenerateInstructionID — identical encoding.
	expected := "0xe5add181f8ca03c9d20dad331b07fb7386d957ca35dcd643bb0a16f48ec0ec09"
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentId := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)
	id, err := instruction.GeneratePayInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId)
	require.NoError(t, err)
	require.Equal(t, expected, id.Hex())
}

func TestGenerateReissueInstructionID(t *testing.T) {
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentId := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)

	// Reissue ID must differ from pay ID for the same paymentId.
	payID, err := instruction.GeneratePayInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId)
	require.NoError(t, err)
	reissueID, err := instruction.GenerateReissueInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId, 1)
	require.NoError(t, err)
	require.NotEqual(t, payID, reissueID)

	// Different reissueNumbers produce different IDs.
	reissueID1, err := instruction.GenerateReissueInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId, 2)
	require.NoError(t, err)
	require.NotEqual(t, reissueID, reissueID1)
}
