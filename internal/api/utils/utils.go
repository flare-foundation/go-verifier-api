package attestationutils

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

/**
 * Encodes attestation type name or source id as a 32-byte hex string.
 * It takes the UTF-8 bytes of the name and pads them with zeros to 32 bytes.
 */
func EncodeAttestationOrSourceName(attestationTypeOrSourceName string) (string, error) {
	if len(attestationTypeOrSourceName) >= 2 && (attestationTypeOrSourceName[:2] == "0x" || attestationTypeOrSourceName[:2] == "0X") {
		return "", fmt.Errorf("attestation type or source id name must not start with '0x'. Provided: '%s'", attestationTypeOrSourceName)
	}
	bytes := []byte(attestationTypeOrSourceName)
	if len(bytes) > 32 {
		return "", fmt.Errorf("attestation type or source id name '%s' is too long (%d bytes)", attestationTypeOrSourceName, len(bytes))
	}
	padded := make([]byte, 32)
	copy(padded, bytes)
	return "0x" + hex.EncodeToString(padded), nil
}

func ParseAttestationType(value string) (connector.AttestationType, error) {
	for _, at := range attestationTypes {
		if string(at) == value {
			return at, nil
		}
	}
	return "", fmt.Errorf("invalid attestation type: %s", value)
}

var attestationTypes = []connector.AttestationType{
	connector.AvailabilityCheck,
	connector.PMWPaymentStatus,
}

func ValidateSystemAndRequestAttestationNameAndSourceId(systemAttestationType connector.AttestationType, systemSourceId string, requestAttestationName string, requestSourceId string) error {
	verifierAttestationNameEnc, err := EncodeAttestationOrSourceName(string(systemAttestationType))
	if err != nil {
		return huma.NewError(http.StatusBadRequest, fmt.Sprintf("attestation type name encoding failed: %v", err))
	}
	verifierSourceNameEnc, err := EncodeAttestationOrSourceName(systemSourceId)
	if err != nil {
		return huma.NewError(http.StatusBadRequest, fmt.Sprintf("source name encoding failed: %v", err))
	}
	if requestAttestationName != verifierAttestationNameEnc || string(requestSourceId) != verifierSourceNameEnc {
		return huma.NewError(http.StatusBadRequest, fmt.Sprintf(
			"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s).",
			requestAttestationName, requestSourceId,
			string(systemAttestationType), verifierAttestationNameEnc,
			systemSourceId, verifierSourceNameEnc,
		))
	}
	return nil
}
