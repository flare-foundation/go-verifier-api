package api

import (
	"context"
	"fmt"
	"io"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/api/handler"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	feeproofservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof"
	multisigservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured"
	paymentservice "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus"
	teeavailabilityservice "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck"
	teeavailabilitycheck "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func LoadModule(ctx context.Context, api huma.API, envConfig config.EnvConfig) ([]io.Closer, error) {
	var closers []io.Closer
	handler.RegisterHealthHandler(api)
	switch envConfig.AttestationType {
	case fdc2.AvailabilityCheck:
		service, err := teeavailabilityservice.NewTeeAvailabilityService(envConfig)
		if err != nil {
			return nil, err
		}
		verifier := service.Verifier()
		config := service.Config()

		handler.RegisterVerificationHandler[fdc2.ITeeAvailabilityCheckRequestBody, fdc2.ITeeAvailabilityCheckResponseBody, types.TeeAvailabilityCheckRequestBody, types.TeeAvailabilityCheckResponseBody](api, &config.EncodedAndABI, verifier)

		// TeeVerifier owns the eth client and CRL cache; needs Close() on shutdown.
		teeVerifier, ok := verifier.(*teeavailabilitycheck.TeeVerifier)
		if !ok {
			logger.Fatalf("Unexpected type for verifier instance")
		}
		closers = append(closers, teeVerifier)
	case fdc2.PMWPaymentStatus:
		service, err := paymentservice.NewPaymentService(envConfig)
		if err != nil {
			return nil, err
		}
		verifier := service.Verifier()
		config := service.Config()

		handler.RegisterVerificationHandler[fdc2.IPMWPaymentStatusRequestBody, fdc2.IPMWPaymentStatusResponseBody, types.PMWPaymentStatusRequestBody, types.PMWPaymentStatusResponseBody](api, &config.EncodedAndABI, verifier)

		closers = append(closers, service)
	case fdc2.PMWMultisigAccountConfigured:
		service, err := multisigservice.NewMultisigService(envConfig)
		if err != nil {
			return nil, err
		}
		verifier := service.Verifier()
		config := service.Config()

		handler.RegisterVerificationHandler[fdc2.IPMWMultisigAccountConfiguredRequestBody, fdc2.IPMWMultisigAccountConfiguredResponseBody, types.PMWMultisigAccountConfiguredRequestBody, types.PMWMultisigAccountConfiguredResponseBody](api, &config.EncodedAndABI, verifier)

	case fdc2.PMWFeeProof:
		service, err := feeproofservice.NewFeeProofService(envConfig)
		if err != nil {
			return nil, err
		}
		verifier := service.Verifier()
		config := service.Config()

		handler.RegisterVerificationHandler[fdc2.IPMWFeeProofRequestBody, fdc2.IPMWFeeProofResponseBody, types.PMWFeeProofRequestBody, types.PMWFeeProofResponseBody](api, &config.EncodedAndABI, verifier)

		closers = append(closers, service)
	default:
		return nil, fmt.Errorf("unsupported attestation type: %s", string(envConfig.AttestationType))
	}
	return closers, nil
}
