package api

import (
	"fmt"
	"log"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	paymentservice "gitlab.com/urskak/verifier-api/internal/attestation/pmw_payment_status"
	"gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/polling"
	teeavailabilitycheck "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/verifier"
	"gitlab.com/urskak/verifier-api/internal/config"
)

func LoadModule(api huma.API, attType connector.AttestationType) error {

	switch attType {
	case connector.PMWPaymentStatus:
		service, err := paymentservice.NewPaymentService()
		if err != nil {
			return fmt.Errorf("failed to initialize payment service: %w", err)
		}
		verifier := service.GetVerifier()
		PMWPaymentStatusHandler(api, connector.PMWPaymentStatus, verifier, service.GetConfig().SourceID)
	case connector.AvailabilityCheck:
		cfg, err := config.GetTeeAvailabilityCheckConfig()
		if err != nil {
			return fmt.Errorf("failed to initialize tee config: %w", err)
		}
		verifier, err := teeavailabilitycheck.GetVerifier(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize tee verifier: %w", err)
		}
		TeeAvailabilityCheckHandler(api, connector.AvailabilityCheck, verifier, cfg.SourceID)
		// Start polling
		teeVerifier, ok := verifier.(*teeavailabilitycheck.TeeVerifier)
		if !ok {
			log.Fatalf("Unexpected type for verifier instance")
		}
		teeVerifier.TeeSamples = make(map[string][]bool)
		go func() {
			ticker := time.NewTicker(polling.SampleInterval)
			defer ticker.Stop()
			for range ticker.C {
				polling.SampleAllTees(teeVerifier)
			}
		}()
	default:
		return fmt.Errorf("unsupported attestation type: %s", attType)
	}
	return nil
}
