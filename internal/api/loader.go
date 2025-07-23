package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/handler"
	paymentservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status"
	teeavailabilityconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/config"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/polling"
	teeavailabilitycheck "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func LoadModule(api huma.API, sourceId config.SourceName, attestationType connector.AttestationType) error {
	switch attestationType {
	case connector.AvailabilityCheck:
		config, err := teeavailabilityconfig.GetTeeAvailabilityCheckConfig(sourceId, attestationType)
		if err != nil {
			return fmt.Errorf("cannot retrieve config %v", err)
		}
		verifier, err := teeavailabilitycheck.GetVerifier(config)
		if err != nil {
			return fmt.Errorf("failed to initialize tee verifier: %w", err)
		}
		handler.TeeAvailabilityCheckHandler(api, *config, verifier)
		// Start polling
		teeVerifier, ok := verifier.(*teeavailabilitycheck.TeeVerifier)
		if !ok {
			log.Fatalf("unexpected type for verifier instance")
		}
		teeVerifier.TeeSamples = make(map[common.Address][]bool)
		go func() {
			ticker := time.NewTicker(polling.SampleInterval)
			defer ticker.Stop()
			for range ticker.C {
				polling.SampleAllTees(context.Background(), teeVerifier)
			}
		}()
	case connector.PMWPaymentStatus:
		service, err := paymentservice.NewPaymentService(sourceId, attestationType)
		if err != nil {
			return fmt.Errorf("%v", err)
		}
		verifier := service.GetVerifier()
		handler.PMWPaymentStatusHandler(api, connector.PMWPaymentStatus, verifier, string(sourceId))
	default:
		return fmt.Errorf("unsupported attestation type: %s", string(attestationType))
	}
	return nil
}
