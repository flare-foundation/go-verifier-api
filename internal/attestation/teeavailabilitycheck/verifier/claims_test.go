package verifier_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	teenodetype "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestValidateClaims(t *testing.T) {
	teeInfoData := teenodetype.TeeInfo{
		Challenge: common.HexToHash("0x1"),
	}
	teeInfoHash, err := teeInfoData.Hash()
	require.NoError(t, err)
	baseClaims := &googlecloud.GoogleTeeClaims{
		HWModel:     "GCP_INTEL_TDX",
		SWName:      "CONFIDENTIAL_SPACE",
		EATNonce:    []string{hex.EncodeToString(teeInfoHash)},
		DebugStatus: "disabled-since-boot",
		SubMods: googlecloud.SubMods{
			ConfidentialSpace: googlecloud.ConfidentialSpaceInfo{
				SupportAttributes: []string{"STABLE"},
			},
			Container: googlecloud.Container{
				ImageDigest: "sha256:194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2",
			},
		},
	}

	t.Run("valid", func(t *testing.T) {
		_, err := verifier.ValidateClaims(baseClaims, teeInfoData, false)
		require.NoError(t, err)
	})
	t.Run("valid in debug mode", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.DebugStatus = "enabled"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, true)
		require.NoError(t, err)
	})
	t.Run("production TEE not allowed in debug mode", func(t *testing.T) {
		_, err := verifier.ValidateClaims(baseClaims, teeInfoData, true)
		require.ErrorContains(t, err, "production TEE not allowed when ALLOW_TEE_DEBUG=true")
	})
	t.Run("debug TEE not allowed in production mode", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.DebugStatus = "enabled"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "not running in production mode")
	})
	t.Run("expect one EATNonce", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.EATNonce = []string{}
		val, err := verifier.ValidateClaims(&modClaims, teenodetype.TeeInfo{}, true)
		require.Equal(t, verifier.StatusInfo{}, val)
		require.ErrorContains(t, err, "expected exactly one EATNonce, got 0")
	})
	t.Run("EATNonce does not match", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.EATNonce = []string{"123"}
		val, err := verifier.ValidateClaims(&modClaims, teenodetype.TeeInfo{}, true)
		require.Equal(t, verifier.StatusInfo{}, val)
		require.ErrorContains(t, err, "EATNonce does not match hash of teeInfo")
	})
	t.Run("no supported attributes", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = nil
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "ConfidentialSpace component has no supported attributes")
	})
	t.Run("empty supported attributes returns OBSOLETE", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = []string{}
		val, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.NoError(t, err)
		require.Equal(t, verifier.OBSOLETE, val.Status)
	})
	t.Run("no confidential space", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SWName = "CONF"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "SWName check failed: expected CONFIDENTIAL_SPACE")
	})
	t.Run("cannot retrieve hash of hwmodel", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.HWModel = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "cannot convert HWMode")
	})
	t.Run("cannot retrieve hash of container.image_digest", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.Container.ImageDigest = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "cannot convert container.image_digest")
	})
}
