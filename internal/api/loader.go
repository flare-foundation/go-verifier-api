package api

import (
	"context"
	"fmt"
	"log"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/handler"
	multisigservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account"
	paymentservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status"
	teeavailabilityconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/config"
	teepoller "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/tee_poller.go" // TODO - why.go?
	teeavailabilitycheck "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func LoadModule(api huma.API, envConfig config.EnvConfig) error {
	handler.RegisterHealthHandler(api)
	switch envConfig.AttestationType {
	case connector.AvailabilityCheck:
		config, err := teeavailabilityconfig.GetTeeAvailabilityCheckConfig(envConfig)
		if err != nil {
			return fmt.Errorf("cannot retrieve config %w", err)
		}
		verifier, err := teeavailabilitycheck.GetVerifier(config)
		if err != nil {
			return fmt.Errorf("failed to initialize TEE verifier: %w", err)
		}
		handler.TeeAvailabilityCheckHandler(api, &config.EncodedAndAbi, verifier)
		// Start poller
		teeVerifier, ok := verifier.(*teeavailabilitycheck.TeeVerifier)
		if !ok {
			log.Fatalf("unexpected type for verifier instance")
		}
		teepoller.StartPoller(context.Background(), teeVerifier)
	case connector.PMWPaymentStatus:
		service, err := paymentservice.NewPaymentService(envConfig)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		verifier := service.GetVerifier()
		config := service.GetConfig()
		handler.PMWPaymentStatusHandler(api, &config.EncodedAndAbi, verifier)
	case connector.PMWMultisigAccountConfigured:
		service, err := multisigservice.NewMultisigService(envConfig)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		verifier := service.GetVerifier()
		config := service.GetConfig()
		handler.PMWMultisigAccountHandler(api, &config.EncodedAndAbi, verifier)
	default:
		return fmt.Errorf("unsupported attestation type: %s", string(envConfig.AttestationType))
	}
	return nil
}
