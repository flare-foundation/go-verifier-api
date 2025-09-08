package api

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/handler"
	multisigservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account"
	paymentservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status"
	teeavailabilityconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/config"
	teepoller "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/tee_poller"
	teeavailabilitycheck "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func LoadModule(ctx context.Context, api huma.API, envConfig config.EnvConfig) ([]io.Closer, error) {
	var closers []io.Closer
	handler.RegisterHealthHandler(api)
	switch envConfig.AttestationType {
	case connector.AvailabilityCheck:
		config, err := teeavailabilityconfig.GetTeeAvailabilityCheckConfig(envConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve config %w", err)
		}
		verifier, err := teeavailabilitycheck.GetVerifier(config)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize TEE verifier: %w", err)
		}
		handler.TeeAvailabilityCheckHandler(api, &config.EncodedAndAbi, verifier)
		// Start poller
		teeVerifier, ok := verifier.(*teeavailabilitycheck.TeeVerifier)
		if !ok {
			log.Fatalf("unexpected type for verifier instance")
		}
		poller := teepoller.StartTeePoller(ctx, teeVerifier)
		closers = append(closers, poller, teeVerifier)
	case connector.PMWPaymentStatus:
		service, err := paymentservice.NewPaymentService(envConfig)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		verifier := service.GetVerifier()
		config := service.GetConfig()
		handler.PMWPaymentStatusHandler(api, &config.EncodedAndAbi, verifier)
		closers = append(closers, service)
	case connector.PMWMultisigAccountConfigured:
		service, err := multisigservice.NewMultisigService(envConfig)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		verifier := service.GetVerifier()
		config := service.GetConfig()
		handler.PMWMultisigAccountHandler(api, &config.EncodedAndAbi, verifier)
	default:
		return nil, fmt.Errorf("unsupported attestation type: %s", string(envConfig.AttestationType))
	}
	return closers, nil
}
