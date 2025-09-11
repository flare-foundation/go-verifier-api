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
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	teeverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func TeeAvailabilityCheckHandler(
	api huma.API,
	config *config.EncodedAndABI,
	verifier verifierinterface.VerifierInterface[
		connector.ITeeAvailabilityCheckRequestBody,
		connector.ITeeAvailabilityCheckResponseBody]) {
	srcID := config.SourceIDPair.SourceID
	attType := config.AttestationTypePair.AttestationType
	tags := getVerifierAPITag(attType)

	RegisterOp(api,
		"post-prepareRequestBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareRequestBody"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequestData[types.TeeAvailabilityCheckRequestBody]
		}) (*types.Response[types.AttestationRequestEncoded], error) {
			err := ValidateRequestData(request.Body, config)
			if err != nil {
				return nil, err
			}
			requestData, err := request.Body.RequestData.ToInternal()
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
			}
			encodedRequest, err := abiEncodeData(requestData, config.ABIPair.Request)
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Encoding request data failed: %v", err))
			}
			return types.NewResponse(types.AttestationRequestEncoded{
				RequestBody: encodedRequest,
			}), nil
		})

	RegisterOp(api,
		"post-prepareResponseBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareResponseBody"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequest
		}) (*types.Response[types.AttestationResponseData[types.TeeAvailabilityCheckResponseBody]], error) {
			err := ValidateRequest(request.Body, config)
			if err != nil {
				return nil, err
			}
			requestData, err := DecodeRequest[connector.ITeeAvailabilityCheckRequestBody](request.Body.RequestBody, config)
			if err != nil {
				return nil, err
			}
			responseData, err := verifier.Verify(ctx, requestData)
			if errors.Is(err, teeverifier.ErrIndeterminate) {
				return nil, huma.Error503ServiceUnavailable(fmt.Sprintf("Verification failed: %v", err))
			}
			if err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", err))
			}
			response, err := EncodeResponse(responseData, config)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.AttestationResponseData[types.TeeAvailabilityCheckResponseBody]{
				ResponseData: types.TeeAvailabilityCheckToExternal(responseData),
				ResponseBody: response,
			}), nil
		})

	RegisterOp(api,
		"post-verify",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "verify"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequest
		}) (*types.Response[types.AttestationResponse], error) {
			logger.Debug("Received request for TEEAvailability")
			err := ValidateRequest(request.Body, config)
			if err != nil {
				return nil, err
			}
			requestData, err := DecodeRequest[connector.ITeeAvailabilityCheckRequestBody](request.Body.RequestBody, config)
			if err != nil {
				return nil, err
			}
			responseData, err := verifier.Verify(ctx, requestData)
			if errors.Is(err, teeverifier.ErrIndeterminate) {
				return nil, huma.Error503ServiceUnavailable(fmt.Sprintf("Verification failed: %v", err))
			} // TODO other errors
			response, err := EncodeResponse(responseData, config)
			if err != nil {
				return nil, err
			}
			logTeeAvailabilityCheckResponse(responseData)
			return types.NewResponse(types.AttestationResponse{
				ResponseBody: response,
			}), nil
		})

	RegisterOp(api,
		"get-polled-tees",
		http.MethodGet,
		"/poller/tees",
		[]string{"Poller"},
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

func formatTeeSamples(teeVerifier *teeverifier.TeeVerifier) []teetype.TeeSample {
	teeVerifier.SamplesMu.RLock()
	defer teeVerifier.SamplesMu.RUnlock()
	samples := make([]teetype.TeeSample, 0, len(teeVerifier.TeeSamples))
	for teeID, values := range teeVerifier.TeeSamples {
		sampleValues := make([]teetype.TeeSampleValue, 0, len(values))
		for _, v := range values {
			sampleValues = append(sampleValues, teetype.TeeSampleValue(v))
		}
		samples = append(samples, teetype.TeeSample{
			TeeID:  teeID.Hex(),
			Values: sampleValues,
		})
	}
	return samples
}
