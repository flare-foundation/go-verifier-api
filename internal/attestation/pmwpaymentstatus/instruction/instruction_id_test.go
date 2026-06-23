package instruction_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGenerateInstructionID(t *testing.T) {
	expected := "0xe5add181f8ca03c9d20dad331b07fb7386d957ca35dcd643bb0a16f48ec0ec09"
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	paymentId := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)
	id, err := instruction.GenerateInstructionID(opTypeBytes, sourceIDBytes, senderAddress, paymentId)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Equal(t, expected, id.Hex())
}
