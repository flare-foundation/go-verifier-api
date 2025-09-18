package instruction_test

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/instruction"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestGenerateChallengeInstructionID(t *testing.T) {
	expected := common.HexToHash("0x7e8040ce6636bb6062419c1560d2cdb0b5ccd4384b6c07e852bfae58426af90d")
	info := testhelper.GetInfoResponse(t)
	challengeHash := common.BytesToHash(info.TeeInfo.Challenge)
	teeID := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	got, err := instruction.GenerateChallengeInstructionID(teeID, challengeHash)
	fmt.Println(got)
	require.NoError(t, err)
	require.Equal(t, expected, got)
}
