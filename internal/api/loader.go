package api

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/gin-gonic/gin"
)

func LoadModule(router *gin.Engine, attType connector.AttestationType) error {
	switch attType {
	case connector.PMWPaymentStatus:
		router.POST("/PMWPaymentStatus/verify", PMWPaymentStatusHandler)
	case connector.AvailabilityCheck:
		router.POST("/TeeAvailabilityCheck/verify", TeeAvailabilityCheckHandler)
	default:
		return fmt.Errorf("unsupported attestation type: %s", attType)
	}
	return nil
}
