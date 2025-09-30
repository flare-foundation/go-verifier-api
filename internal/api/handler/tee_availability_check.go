package handler

import (
	"context"
	"errors"
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
			err := ValidateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				logger.Errorf("Request validation failed: %v", err)
				return nil, huma.Error400BadRequest("Request validation failed: " + err.Error())
			}
			encodedRequest, err := PrepareRequestBody(request.Body, config)
			if err != nil {
				logger.Errorf("%v", err)
				return nil, huma.Error400BadRequest("Prepare request failed: " + err.Error())
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
			err := ValidateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				logger.Errorf("Request validation failed: %v", err)
				return nil, huma.Error400BadRequest("Request validation failed: " + err.Error())
			}
			requestData, err := DecodeRequest[connector.ITeeAvailabilityCheckRequestBody](request.Body.RequestBody, config)
			if err != nil {
				logger.Errorf("Decoding request body to data failed: %v", err)
				return nil, huma.Error400BadRequest("Decoding request body to data failed: " + err.Error())
			}
			responseData, err := verifier.Verify(ctx, requestData)
			if err != nil {
				logger.Errorf("Verification failed: %v", err)
				return nil, huma.Error500InternalServerError("Verification failed: " + err.Error())
			}
			encodedResponse, err := EncodeResponse(responseData, config)
			if err != nil {
				logger.Errorf("Encoding data to response body failed: %v", err)
				return nil, huma.Error500InternalServerError("Encoding data to response body failed: " + err.Error())
			}
			return types.NewResponse(types.AttestationResponseData[types.TeeAvailabilityCheckResponseBody]{
				ResponseData: types.TeeAvailabilityCheckResponseToExternal(responseData),
				ResponseBody: encodedResponse,
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
			logger.Debug("Received request for TEEAvailabilityCheck")
			err := ValidateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				logger.Errorf("Request validation failed: %v", err)
				return nil, huma.Error400BadRequest("Request validation failed: " + err.Error())
			}
			requestData, err := DecodeRequest[connector.ITeeAvailabilityCheckRequestBody](request.Body.RequestBody, config)
			if err != nil {
				logger.Errorf("Decoding request body to data failed: %v", err)
				return nil, huma.Error400BadRequest("Decoding request body to data failed: " + err.Error())
			}
			responseData, err := verifier.Verify(ctx, requestData)
			if err != nil {
				logger.Errorf("Verification failed: %v", err)
				if errors.Is(err, teeverifier.ErrIndeterminate) {
					return nil, huma.Error503ServiceUnavailable("Verification cannot be determinate: " + err.Error())
				} else {
					return nil, huma.Error500InternalServerError("Verification failed: " + err.Error())
				}
			}
			encodedResponse, err := EncodeResponse(responseData, config)
			if err != nil {
				logger.Errorf("Encoding data to response body failed: %v", err)
				return nil, huma.Error500InternalServerError("Encoding data to response body failed: " + err.Error())
			}
			logTeeAvailabilityCheckResponse(responseData)
			return types.NewResponse(types.AttestationResponse{
				ResponseBody: encodedResponse,
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
				logger.Errorf("Internal server error: invalid verifier type")
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
