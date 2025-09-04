package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/types"
	teeverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func TeeAvailabilityCheckHandler(
	api huma.API,
	config *config.EncodedAndAbi,
	verifier verifierinterface.VerifierInterface[
		connector.ITeeAvailabilityCheckRequestBody,
		connector.ITeeAvailabilityCheckResponseBody]) {
	srcID := config.SourceIdPair.SourceId
	attType := config.AttestationTypePair.AttestationType
	tags := getVerifierAPITag(attType)

	RegisterOp(api,
		"post-prepareRequestBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareRequestBody"),
		tags,
		false,
		func(ctx context.Context, request *struct {
			Body types.TeeAvailabilityRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validatePrepareResponseBody[types.TeeAvailabilityRequestBody](request.Body, config); err != nil {
				return nil, err
			}
			requestData, err := request.Body.RequestData.ToInternal()
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
			}
			return prepareRequestBody[connector.ITeeAvailabilityCheckRequestBody](requestData, config)
		})

	RegisterOp(api,
		"post-prepareResponseBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareResponseBody"),
		tags,
		false,
		func(ctx context.Context, request *struct {
			Body types.FTDCRequestEncoded
		}) (*types.Response[types.RawAndEncodedTeeAvailabilityResponseBody], error) {
			return prepareResponseBody(
				ctx,
				request.Body,
				validateAndVerifyEncodedRequest,
				types.TeeToExternal,
				config,
				verifier,
			)
		})

	RegisterOp(api,
		"post-verify",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "verify"),
		tags,
		true,
		func(ctx context.Context, request *struct {
			Body connector.IFtdcHubFtdcAttestationRequest
		}) (*types.Response[types.EncodedResponseBody], error) {
			logger.Debug("Received request for TEEAvailability")
			response, responseDataBytes, err := validateAndVerifyEncodedRequest(request.Body, ctx, config, verifier)
			if err != nil {
				logger.Error("Failed verifying request", err)
				return nil, err
			}
			logTeeAvailabilityCheckResponse(response)
			return types.NewResponse(types.EncodedResponseBody{
				Response: responseDataBytes,
			}), nil
		})

	RegisterOp(api,
		"get-polled-tees",
		http.MethodGet,
		"/poller/tees",
		[]string{"Poller"},
		false,
		func(ctx context.Context, request *struct{}) (*types.Response[types.TeeSamplesResponse], error) {
			teeVerifier, ok := verifier.(*teeverifier.TeeVerifier)
			if !ok {
				return nil, huma.NewError(
					http.StatusInternalServerError,
					"Internal server error: invalid verifier type",
				)
			}
			samples := formatTeeSamples(teeVerifier)
			return types.NewResponse(types.TeeSamplesResponse{Samples: samples}), nil
		})
}

func validateAndVerifyEncodedRequest(request connector.IFtdcHubFtdcAttestationRequest, ctx context.Context, config *config.EncodedAndAbi, verifier verifierinterface.VerifierInterface[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody]) (connector.ITeeAvailabilityCheckResponseBody, []byte, error) {
	requestData, err := validateAndParseFTDCRequest[connector.ITeeAvailabilityCheckRequestBody](request, config)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, err
	}
	responseData, err := verifier.Verify(ctx, requestData)
	if errors.Is(err, teeverifier.ErrIndeterminate) {
		return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, huma.Error503ServiceUnavailable(fmt.Sprintf("Verification failed: %v", err))
	}
	return handleVerifierResult[connector.ITeeAvailabilityCheckResponseBody](err, responseData, config)
}

func formatTeeSamples(teeVerifier *teeverifier.TeeVerifier) []teetypes.TeeSample {
	teeVerifier.SamplesMu.RLock()
	defer teeVerifier.SamplesMu.RUnlock()
	samples := make([]teetypes.TeeSample, 0, len(teeVerifier.TeeSamples))
	for teeID, values := range teeVerifier.TeeSamples {
		sampleValues := make([]teetypes.TeeSampleValue, 0, len(values))
		for _, v := range values {
			sampleValues = append(sampleValues, teetypes.TeeSampleValue(v))
		}
		samples = append(samples, teetypes.TeeSample{
			TeeID:  teeID.Hex(),
			Values: sampleValues,
		})
	}
	return samples
}
