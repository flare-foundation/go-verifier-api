package verifier

import (
	"encoding/hex"
	"errors"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"

	teenodetype "github.com/flare-foundation/tee-node/pkg/types"
)

type AvailabilityCheckStatus uint8

const (
	OK AvailabilityCheckStatus = iota
	OBSOLETE
	DOWN
)

type StatusInfo struct {
	CodeHash common.Hash
	Platform common.Hash
	Status   AvailabilityCheckStatus
}

func ValidateClaims(claims *googlecloud.GoogleTeeClaims, teeInfoData teenodetype.TeeInfo, allowDebugMode bool) (StatusInfo, error) {
	var statusInfo StatusInfo
	if len(claims.EATNonce) != 1 {
		return StatusInfo{}, fmt.Errorf("expected exactly one EATNonce, got %d", len(claims.EATNonce))
	}
	// generate teeInfo hash
	teeInfoBytes, err := teeInfoData.Hash()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot create hash of teeInfo: %w", err)
	}
	// match with eat_nonce
	if claims.EATNonce[0] != hex.EncodeToString(teeInfoBytes) {
		return StatusInfo{}, errors.New("EATNonce does not match hash of teeInfo")
	}
	// Check if running in production. Allow debug mode only if ALLOW_TEE_DEBUG is enabled.
	if allowDebugMode {
		if claims.DebugStatus == "disabled-since-boot" {
			return StatusInfo{}, errors.New("production TEE not allowed when ALLOW_TEE_DEBUG=true")
		}
		// No check for supported attributes in debug mode
		statusInfo.Status = OK
	} else {
		// Non-debug mode
		if claims.DebugStatus != "disabled-since-boot" {
			return StatusInfo{}, errors.New("TEE is not running in production mode")
		}
		// Check Confidential Space image version
		if claims.SubMods.ConfidentialSpace.SupportAttributes == nil {
			return StatusInfo{}, errors.New("ConfidentialSpace component has no supported attributes")
		}
		if slices.Contains(claims.SubMods.ConfidentialSpace.SupportAttributes, "STABLE") {
			statusInfo.Status = OK
		} else {
			statusInfo.Status = OBSOLETE
		}
	}
	// Check the OS is Confidential Space
	if claims.SWName != "CONFIDENTIAL_SPACE" {
		return StatusInfo{}, fmt.Errorf("SWName check failed: expected CONFIDENTIAL_SPACE, got %q", claims.SWName)
	}
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
