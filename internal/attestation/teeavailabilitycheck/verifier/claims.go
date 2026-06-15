package verifier

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
)

type AvailabilityCheckStatus uint8

const (
	OK AvailabilityCheckStatus = iota
	OBSOLETE
)

type StatusInfo struct {
	CodeHash common.Hash
	Platform common.Hash
	Status   AvailabilityCheckStatus
}

// ValidateClaims performs the verifier-specific checks left after
// googlecloud.ParseAndValidatePKIToken applies the Policy. The Policy
// already enforces JWT signature, iss/aud, eat_nonce binding, and (when
// AllowDebug is false) the dbgstat allowlist. This function covers what the
// Policy does not: SWName must be CONFIDENTIAL_SPACE, the STABLE
// support_attribute downgrade-to-OBSOLETE rule on the production path, and
// extraction of CodeHash (from container.image_id) and Platform. The image_id
// is not allowlisted here — the accepted CodeHash is enforced on-chain.
func ValidateClaims(claims *googlecloud.GoogleTeeClaims) (StatusInfo, error) {
	var statusInfo StatusInfo
	if claims.SWName != "CONFIDENTIAL_SPACE" {
		return StatusInfo{}, fmt.Errorf("SWName check failed: expected CONFIDENTIAL_SPACE, got %q", claims.SWName)
	}
	// When the Policy ran with AllowDebug=false the token was rejected
	// unless dbgstat == "disabled-since-boot", so any claims reaching
	// here in that mode describe a production TEE. A non-production
	// dbgstat is therefore reachable only when ALLOW_TEE_DEBUG=true.
	isProduction := claims.DebugStatus == "disabled-since-boot"
	if isProduction {
		if claims.SubMods.ConfidentialSpace.SupportAttributes == nil {
			return StatusInfo{}, errors.New("ConfidentialSpace component has no supported attributes")
		}
		if slices.Contains(claims.SubMods.ConfidentialSpace.SupportAttributes, "STABLE") {
			statusInfo.Status = OK
		} else {
			statusInfo.Status = OBSOLETE
		}
	} else {
		logger.Warnf("admitting debug-mode TEE because ALLOW_TEE_DEBUG=true (dbgstat=%q). Do not use in production.", claims.DebugStatus)
		statusInfo.Status = OK
	}
	var err error
	statusInfo.CodeHash, err = claims.CodeHash()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot convert container.image_id %q to Bytes32: %w", claims.SubMods.Container.ImageID, err)
	}
	statusInfo.Platform, err = claims.Platform()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot convert HWModel %s to Bytes32: %w", claims.HWModel, err)
	}
	return statusInfo, nil
}
