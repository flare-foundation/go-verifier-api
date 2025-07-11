package validation

import (
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-playground/validator/v10"
)

func IsHash32(fl validator.FieldLevel) bool {
	hash := fl.Field().String()
	re := regexp.MustCompile(`^0x[a-fA-F0-9]{64}$`)
	return re.MatchString(hash)
}

func IsCommonAddress(fl validator.FieldLevel) bool {
	return common.IsHexAddress(fl.Field().String())
}
