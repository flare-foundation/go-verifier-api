package instruction_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_fee_proof/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGeneratePayInstructionID(t *testing.T) {
	// Same expected value as PMWPaymentStatus GenerateInstructionID — identical encoding.
	expected := "0xaf551716a458aae6b63fad419f4f467fd588c7c2ad921bf2e8a2a52ee16215aa"
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	nonce := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)
	id, err := instruction.GeneratePayInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce)
	require.NoError(t, err)
	require.Equal(t, expected, id.Hex())
}

func TestGenerateReissueInstructionID(t *testing.T) {
	senderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	nonce := uint64(11263155)
	opTypeBytes, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)
	sourceIDBytes, err := convert.StringToCommonHash(string(config.SourceTestXRP))
	require.NoError(t, err)

	// Reissue ID must differ from pay ID for the same nonce.
	payID, err := instruction.GeneratePayInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce)
	require.NoError(t, err)
	reissueID, err := instruction.GenerateReissueInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce, 0)
	require.NoError(t, err)
	require.NotEqual(t, payID, reissueID)

	// Different reissueNumbers produce different IDs.
	reissueID1, err := instruction.GenerateReissueInstructionID(opTypeBytes, sourceIDBytes, senderAddress, nonce, 1)
	require.NoError(t, err)
	require.NotEqual(t, reissueID, reissueID1)
}
