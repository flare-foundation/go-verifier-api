package handler

import (
	"encoding/hex"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/go-playground/validator/v10"
	"gitlab.com/urskak/verifier-api/internal/api/validation"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("hash32", validation.IsHash32)
	validate.RegisterValidation("eth_addr", validation.IsCommonAddress)
}

func ValidateRequest(request interface{}) error {
	if err := validate.Struct(request); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("validation failed: %v", err))
	}
	return nil
}

func ValidateSystemAndRequestAttestationNameAndSourceId(systemAttestationType connector.AttestationType, systemSourceId string, requestAttestationName string, requestSourceId string) error {
	verifierAttestationNameEnc, err := encodeAttestationOrSourceName(string(systemAttestationType))
	if err != nil {
		return huma.Error500InternalServerError(fmt.Sprintf("system attestation type name encoding failed: %v", err))
	}
	verifierSourceNameEnc, err := encodeAttestationOrSourceName(systemSourceId)
	if err != nil {
		return huma.Error500InternalServerError(fmt.Sprintf("system source name encoding failed: %v", err))
	}
	if requestAttestationName != verifierAttestationNameEnc || string(requestSourceId) != verifierSourceNameEnc {
		return huma.Error400BadRequest(fmt.Sprintf(
			"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s).",
			requestAttestationName, requestSourceId,
			string(systemAttestationType), verifierAttestationNameEnc,
			systemSourceId, verifierSourceNameEnc,
		))
	}
	return nil
}

func encodeAttestationOrSourceName(attestationTypeOrSourceName string) (string, error) {
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

func HexWith0x(data []byte) string {
	return "0x" + hex.EncodeToString(data)
}
