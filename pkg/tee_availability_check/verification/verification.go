package verification

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"gitlab.com/urskak/verifier-api/pkg/tee_availability_check/types"
)

func VerifyAttestation(attestationToken string, infoData types.ProxyInfoData) (connector.ITeeAvailabilityCheckResponseBody, error) {
	cert, err := LoadRootCert()
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to load root cert: %w", err)
	}
	token, err := ValidatePKIToken(cert, attestationToken)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to load root cert: %w", err)
	}
	if !token.Valid {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("attestation token is invalid: %s", attestationToken)
	}
	statusInfo, err := ValidateClaims(token, infoData)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, fmt.Errorf("failed to validate claims: %w", err)
	}
	var responseBody connector.ITeeAvailabilityCheckResponseBody
	responseBody.Status = uint8(statusInfo.Status)
	responseBody.CodeHash = statusInfo.CodeHash
	responseBody.Platform = statusInfo.Platform
	responseBody.MachineStatus = uint8(infoData.Status)
	responseBody.TeeTimestamp = infoData.TeeTimestamp
	responseBody.InitialTeeId = infoData.InitialTeeId
	responseBody.TeeGovernanceHash = infoData.TeeGovernanceHash
	responseBody.RewardEpochId = infoData.LastSigningPolicyId

	return responseBody, nil
}
