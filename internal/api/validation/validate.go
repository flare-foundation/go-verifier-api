package validation

import (
	"regexp"

	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("hash32", IsHash32)
	validate.RegisterValidation("eth_addr", IsCommonAddress)
}

func ValidateRequest(request interface{}) error {
	if err := validate.Struct(request); err != nil {
		return err
	}
	return nil
}

func ValidateSystemAndRequestAttestationNameAndSourceId(attestationTypePair config.AttestationTypeEncodedPair, sourceIdPair config.SourceIdEncodedPair, requestAttestationName string, requestSourceId string) error {
	if requestAttestationName != attestationTypePair.AttestationTypeEncoded || string(requestSourceId) != sourceIdPair.SourceIdEncoded {
		return fmt.Errorf(
			"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s)",
			requestAttestationName, requestSourceId,
			string(attestationTypePair.AttestationType), attestationTypePair.AttestationTypeEncoded,
			sourceIdPair.SourceId, sourceIdPair.SourceIdEncoded,
		)
	}
	return nil
}

func IsHash32(fl validator.FieldLevel) bool {
	hash := fl.Field().String()
	re := regexp.MustCompile(`^0x[a-fA-F0-9]{64}$`)
	return re.MatchString(hash)
}

func IsCommonAddress(fl validator.FieldLevel) bool {
	return common.IsHexAddress(fl.Field().String())
}
