package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/go-playground/validator/v10"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
	attestationutils "gitlab.com/urskak/verifier-api/internal/api/utils"
	"gitlab.com/urskak/verifier-api/internal/api/validation"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("hash32", validation.IsHash32)
	validate.RegisterValidation("eth_addr", validation.IsCommonAddress)
}

func PMWPaymentStatusHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[attestationtypes.IPMWPaymentStatusRequestBody, attestationtypes.IPMWPaymentStatusResponseBody], sourceID string) {

	huma.Register(api, huma.Operation{
		OperationID:   fmt.Sprintf("postVerify_%s", attestationType),
		Summary:       fmt.Sprintf("Attestation for %s", attestationType),
		Method:        http.MethodPost,
		Path:          fmt.Sprintf("/%s/verify", attestationType),
		Tags:          []string{string(attestationType)},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, request *attestationtypes.AttestationRequestPMWPaymentStatus) (*attestationtypes.FullAttestationResponsePMWPaymentStatus, error) {
		fmt.Println(request)
		if err := validate.Struct(request); err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
		}
		verifierAttestationNameEnc, err := attestationutils.EncodeAttestationOrSourceName(string(attestationType))
		if err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("attestation type name encoding failed: %v", err))
		}
		verifierSourceNameEnc, err := attestationutils.EncodeAttestationOrSourceName(sourceID)
		if err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("source name encoding failed: %v", err))
		}
		if request.AttestationType != verifierAttestationNameEnc || request.SourceID != verifierSourceNameEnc {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf(
				"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s).",
				request.AttestationType, request.SourceID,
				string(attestationType), verifierAttestationNameEnc,
				sourceID, verifierSourceNameEnc,
			))
		}

		status, res, err := verifier.Verify(ctx, request.RequestBody)
		response := &attestationtypes.AttestationResponsePMWPaymentStatus{
			AttestationType: request.AttestationType,
			SourceID:        request.SourceID,
			RequestBody:     request.RequestBody,
			ResponseBody:    res,
		}

		return &attestationtypes.FullAttestationResponsePMWPaymentStatus{
			AttestationStatus: string(status),
			Response:          response,
		}, err // TODO separate error and none error - check underlying code
	})
}

func TeeAvailabilityCheckHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[attestationtypes.ITeeAvailabilityCheckRequestBody, attestationtypes.ITeeAvailabilityCheckResponseBody], sourceID string) {

	huma.Register(api, huma.Operation{
		OperationID:   fmt.Sprintf("postVerify_%s", attestationType),
		Summary:       fmt.Sprintf("Attestation for %s", attestationType),
		Method:        http.MethodPost,
		Path:          fmt.Sprintf("/%s/verify", attestationType),
		Tags:          []string{string(attestationType)},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, request *attestationtypes.AttestationRequestTeeAvailabilityCheck) (*attestationtypes.FullAttestationResponseTeeAvailabilityCheck, error) {
		fmt.Println(request)
		if err := validate.Struct(request); err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
		}
		verifierAttestationNameEnc, err := attestationutils.EncodeAttestationOrSourceName(string(attestationType))
		if err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("attestation type name encoding failed: %v", err))
		}
		verifierSourceNameEnc, err := attestationutils.EncodeAttestationOrSourceName(sourceID)
		if err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("source name encoding failed: %v", err))
		}
		if request.Body.AttestationType != verifierAttestationNameEnc || request.Body.SourceID != verifierSourceNameEnc {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf(
				"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s).",
				request.Body.AttestationType, request.Body.SourceID,
				string(attestationType), verifierAttestationNameEnc,
				sourceID, verifierSourceNameEnc,
			))
		}

		status, res, err := verifier.Verify(ctx, request.Body.RequestBody)
		response := &attestationtypes.AttestationResponseTeeAvailabilityCheck{
			AttestationType: request.Body.AttestationType,
			SourceID:        request.Body.SourceID,
			RequestBody:     request.Body.RequestBody,
			ResponseBody:    res,
		}

		return &attestationtypes.FullAttestationResponseTeeAvailabilityCheck{
			AttestationStatus: string(status),
			Response:          response,
		}, err // TODO separate error and none error - check underlying code
	})
}
