package instruction_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/instruction"
	"github.com/stretchr/testify/require"
)

func TestGenerateChallengeInstructionID(t *testing.T) {
	expected := common.HexToHash("0x7e8040ce6636bb6062419c1560d2cdb0b5ccd4384b6c07e852bfae58426af90d")
	challengeHash := common.HexToHash("0xc1cb2b6251fd40dee87e556828e05636c8c4019bc8accb6dbacbe8114a3595c7")
	teeID := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	got, err := instruction.GenerateChallengeInstructionID(teeID, challengeHash)
	require.NoError(t, err)
	require.Equal(t, expected, got)
}
