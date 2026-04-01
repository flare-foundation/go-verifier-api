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
	expected := "0xaf551716a458aae6b63fad419f4f467fd588c7c2ad921bf2e8a2a52ee16215aa"
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	nonce := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)
	id, err := instruction.GenerateInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Equal(t, expected, id.Hex())
}
