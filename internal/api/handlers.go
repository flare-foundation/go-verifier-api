package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	paymentservice "gitlab.com/urskak/verifier-api/internal/attestation/pmw_payment_status"
	teeavailabilitycheck "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/verifier"
	"gitlab.com/urskak/verifier-api/internal/config"
)

// @Summary Verify PMW Payment Status
// @Description Verifies PMW payment status attestation
// @Accept json
// @Produce json
// @Param request body attestationtypes.AttestationRequestPMWPaymentStatus true "Attestation Request"
// @Success 200 {object} attestationtypes.FullAttestationResponsePMWPaymentStatus
// @Failure 400 {object} map[string]string
// @Router /PMWPaymentStatus/verify [post]
func PMWPaymentStatusHandler(c *gin.Context) {
	service, err := paymentservice.NewPaymentService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize service"})
		return
	}
	verifier := service.GetVerifier()

	genericVerifyHandler(c, verifier)
}

// @Summary Verify TEE Availability Check
// @Description Verifies TEE availability check attestation
// @Accept json
// @Produce json
// @Param request body attestationtypes.AttestationRequestTeeAvailabilityCheck true "Attestation Request"
// @Success 200 {object} attestationtypes.FullAttestationResponseTeeAvailabilityCheck
// @Failure 400 {object} map[string]string
// @Router /TeeAvailabilityCheck/verify [post]
func TeeAvailabilityCheckHandler(c *gin.Context) {
	cfg, err := config.GetTeeAvailabilityCheckConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load config"})
		return
	}
	verifier, err := teeavailabilitycheck.GetVerifier(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize verifier"})
		return
	}

	genericVerifyHandler(c, verifier)
}
