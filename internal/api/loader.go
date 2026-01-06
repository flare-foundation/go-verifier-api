package api

import (
	"context"
	"fmt"
	"io"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/handler"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	multisigservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured"
	paymentservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status"
	teeavailabilityservice "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check"
	teepoller "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/tee_poller"
	teeavailabilitycheck "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func LoadModule(ctx context.Context, api huma.API, envConfig config.EnvConfig) ([]io.Closer, error) {
	var closers []io.Closer
	handler.RegisterHealthHandler(api)
	switch envConfig.AttestationType {
	case connector.AvailabilityCheck:
		service, err := teeavailabilityservice.NewTeeAvailabilityService(envConfig)
		if err != nil {
			return nil, err
		}
		verifier := service.GetVerifier()
		config := service.GetConfig()

		handler.RegisterVerificationHandler[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody, types.TeeAvailabilityCheckRequestBody, types.TeeAvailabilityCheckResponseBody](api, &config.EncodedAndABI, verifier)

		// Start poller
		teeVerifier, ok := verifier.(*teeavailabilitycheck.TeeVerifier)
		if !ok {
			logger.Fatalf("Unexpected type for verifier instance")
		}
		handler.RegisterTeePoolingHandler(api, teeVerifier)

		poller := teepoller.NewTeePoller(ctx, teeVerifier)
		poller.StartTeePoller()

		closers = append(closers, poller, teeVerifier)
	case connector.PMWPaymentStatus:
		service, err := paymentservice.NewPaymentService(envConfig)
		if err != nil {
			return nil, err
		}
		verifier := service.GetVerifier()
		config := service.GetConfig()

		handler.RegisterVerificationHandler[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody, types.PMWPaymentStatusRequestBody, types.PMWPaymentStatusResponseBody](api, &config.EncodedAndABI, verifier)

		closers = append(closers, service)
	case connector.PMWMultisigAccountConfigured:
		service, err := multisigservice.NewMultisigService(envConfig)
		if err != nil {
			return nil, err
		}
		verifier := service.GetVerifier()
		config := service.GetConfig()

		handler.RegisterVerificationHandler[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody, types.PMWMultisigAccountConfiguredRequestBody, types.PMWMultisigAccountConfiguredResponseBody](api, &config.EncodedAndABI, verifier)

	default:
		return nil, fmt.Errorf("unsupported attestation type: %s", string(envConfig.AttestationType))
	}
	return closers, nil
}
