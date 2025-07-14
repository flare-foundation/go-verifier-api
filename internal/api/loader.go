package api

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/gin-gonic/gin"
	"gitlab.com/urskak/verifier-api/internal/api/middleware"
)

func LoadModule(router *gin.Engine, attType connector.AttestationType) error {
	protected := router.Group("/")
	protected.Use(middleware.ApiKeyAuthMiddleware())
	switch attType {
	case connector.PMWPaymentStatus:
		protected.POST("/PMWPaymentStatus/verify", PMWPaymentStatusHandler)
	case connector.AvailabilityCheck:
		protected.POST("/TeeAvailabilityCheck/verify", TeeAvailabilityCheckHandler)
	default:
		return fmt.Errorf("unsupported attestation type: %s", attType)
	}
	return nil
}
