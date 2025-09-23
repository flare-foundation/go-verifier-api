package verifier_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/tee"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	teenodetype "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestTeeInfoHash(t *testing.T) {
	var x [32]byte
	var y [32]byte
	for i := 0; i < 32; i++ {
		x[i] = byte(i)
		y[i] = byte(32 - i)
	}

	mockPublicKey := tee.PublicKey{
		X: x,
		Y: y,
	}
	systemState, err := hexutil.Decode("0x")
	require.NoError(t, err)
	state, err := hexutil.Decode("0x")
	require.NoError(t, err)
	mockData := tee.TeeStructsAttestation{
		Challenge:                common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000001234"),
		PublicKey:                mockPublicKey,
		InitialSigningPolicyId:   1,
		InitialSigningPolicyHash: common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000abcd"),
		LastSigningPolicyId:      2,
		LastSigningPolicyHash:    common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000dead"),
		State: tee.ITeeAvailabilityCheckTeeState{
			SystemState:        systemState,
			SystemStateVersion: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			State:              state,
			StateVersion:       common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		},
		TeeTimestamp: 123456789,
	}
	hash, err := verifier.TeeInfoHash(mockData)
	require.NoError(t, err)
	require.NotEmpty(t, hash)
	_, err = hex.DecodeString(hash)
	require.NoError(t, err)
}

func TestValidateClaims(t *testing.T) {
	teeInfoData := teenodetype.TeeInfo{
		Challenge: common.HexToHash("0x1"),
	}
	teeInfoHash, err := teeInfoData.Hash()
	require.NoError(t, err)
	baseClaims := &teetype.GoogleTeeClaims{
		HWModel:     "GCP_INTEL_TDX",
		SWName:      "CONFIDENTIAL_SPACE",
		EATNonce:    []string{hex.EncodeToString(teeInfoHash)},
		DebugStatus: "disabled-since-boot",
		SubMods: teetype.SubModules{
			ConfidentialSpace: teetype.ConfidentialSpaceInfo{
				SupportAttributes: []string{"STABLE"},
			},
			Container: teetype.Container{
				ImageDigest: "sha256:0f5455255ce543c2fa319153577e2ad75d7f8ea698df1cab1a8c782b391b6354",
			},
		},
	}

	t.Run("Valid claims", func(t *testing.T) {
		_, err := verifier.ValidateClaims(baseClaims, teeInfoData, false)
		require.NoError(t, err)
	})
	t.Run("Valid claims in debug mode", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.DebugStatus = "enabled"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, true)
		require.NoError(t, err)
	})
	t.Run("Production TEE not allowed in debug mode", func(t *testing.T) {
		_, err := verifier.ValidateClaims(baseClaims, teeInfoData, true)
		require.ErrorContains(t, err, "production TEE not allowed when ALLOW_TEE_DEBUG=true")
	})
	t.Run("Debug TEE not allowed in production mode", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.DebugStatus = "enabled"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "not running in production mode")
	})
	t.Run("Expect one EATNonce", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.EATNonce = []string{}
		val, err := verifier.ValidateClaims(&modClaims, teenodetype.TeeInfo{}, true)
		var zero teetype.StatusInfo
		require.Equal(t, zero, val)
		require.ErrorContains(t, err, "expected one eat_nonce")
	})
	t.Run("EATNonce does not match", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.EATNonce = []string{"123"}
		val, err := verifier.ValidateClaims(&modClaims, teenodetype.TeeInfo{}, true)
		var zero teetype.StatusInfo
		require.Equal(t, zero, val)
		require.ErrorContains(t, err, "eat_nonce does not match")
	})
	t.Run("No supported attributes", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = nil
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "no supported attributes found")
	})
	t.Run("No supported attributes", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = []string{}
		val, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.NoError(t, err)
		require.Equal(t, val.Status, teetype.OBSOLETE)
	})
	t.Run("No confidential space", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SWName = "CONF"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "not running in CONFIDENTIAL_SPACE")
	})
	t.Run("Cannot retrieve hash of hwmodel", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.HWModel = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "cannot retrieve hash of hwmodel")
	})
	t.Run("Cannot retrieve hash of container.image_digest", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.Container.ImageDigest = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "cannot retrieve hash of container.image_digest")
	})
}
