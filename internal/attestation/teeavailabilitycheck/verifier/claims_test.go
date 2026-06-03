package verifier_test

import (
	"strings"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	"github.com/stretchr/testify/require"
)

func TestValidateClaims(t *testing.T) {
	baseClaims := &googlecloud.GoogleTeeClaims{
		HWModel:     "GCP_INTEL_TDX",
		SWName:      "CONFIDENTIAL_SPACE",
		DebugStatus: "disabled-since-boot",
		SubMods: googlecloud.SubMods{
			ConfidentialSpace: googlecloud.ConfidentialSpaceInfo{
				SupportAttributes: []string{"STABLE"},
			},
			Container: googlecloud.Container{
				ImageID: "sha256:194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2",
			},
		},
	}

	t.Run("prod TEE with STABLE returns OK", func(t *testing.T) {
		val, err := verifier.ValidateClaims(baseClaims)
		require.NoError(t, err)
		require.Equal(t, verifier.OK, val.Status)
	})
	t.Run("debug TEE bypasses STABLE check", func(t *testing.T) {
		// Reached only when ParseAndValidatePKIToken ran with AllowDebug=true,
		// so STABLE downgrade is intentionally skipped.
		modClaims := *baseClaims
		modClaims.DebugStatus = "enabled"
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = nil
		val, err := verifier.ValidateClaims(&modClaims)
		require.NoError(t, err)
		require.Equal(t, verifier.OK, val.Status)
	})
	t.Run("prod TEE without STABLE downgrades to OBSOLETE", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = []string{}
		val, err := verifier.ValidateClaims(&modClaims)
		require.NoError(t, err)
		require.Equal(t, verifier.OBSOLETE, val.Status)
	})
	t.Run("prod TEE with nil supported attributes errors", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = nil
		_, err := verifier.ValidateClaims(&modClaims)
		require.ErrorContains(t, err, "ConfidentialSpace component has no supported attributes")
	})
	t.Run("SWName must be CONFIDENTIAL_SPACE", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SWName = "CONF"
		_, err := verifier.ValidateClaims(&modClaims)
		require.ErrorContains(t, err, "SWName check failed: expected CONFIDENTIAL_SPACE")
	})
	t.Run("invalid hwmodel rejected", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.HWModel = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims)
		require.ErrorContains(t, err, "cannot convert HWMode")
	})
	t.Run("invalid container.image_id rejected", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.Container.ImageID = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims)
		require.ErrorContains(t, err, "cannot convert container.image_id")
	})
}
