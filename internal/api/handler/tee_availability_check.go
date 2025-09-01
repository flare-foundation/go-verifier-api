package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	teeverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func TeeAvailabilityCheckHandler(
	api huma.API,
	config *config.EncodedAndAbi,
	verifier verifierinterface.VerifierInterface[
		connector.ITeeAvailabilityCheckRequestBody,
		connector.ITeeAvailabilityCheckResponseBody]) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        getVerifierAPIPath(config.SourceIdPair.SourceId, config.AttestationTypePair.AttestationType, "prepareRequestBody"),
		Tags:        getVerifierAPITag(config.AttestationTypePair.AttestationType)},
		func(ctx context.Context, request *struct {
			Body types.TeeAvailabilityRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validation.ValidateRequest(request); err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
			}
			if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(config.AttestationTypePair, config.SourceIdPair, request.Body.FTDCHeader.AttestationType, request.Body.FTDCHeader.SourceId); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
			}
			requestData, err := request.Body.RequestData.ToInternal()
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
			}
			// TODO-later add validation (later, now just use it as a helper to generate abi encoded request)
			requestDataBytes, err := utils.AbiEncodeData[connector.ITeeAvailabilityCheckRequestBody](requestData, config.AbiPair.Request)
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Encoding request data failed: %v", err))
			}
			return types.NewResponse(types.EncodedRequestBody{
				RequestBody: utils.BytesToHex0x(requestDataBytes),
			}), nil
		})
	// prepare ResponseBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareResponseBody",
		Method:      http.MethodPost,
		Path:        getVerifierAPIPath(config.SourceIdPair.SourceId, config.AttestationTypePair.AttestationType, "prepareResponseBody"),
		Tags:        getVerifierAPITag(config.AttestationTypePair.AttestationType)},
		func(ctx context.Context, request *struct {
			Body types.FTDCRequestEncoded
		}) (*types.Response[types.RawAndEncodedTeeAvailabilityResponseBody], error) {
			attestationRequest, err := toIFTdcHubFtdcAttestationRequest(request.Body)
			if err != nil {
				return nil, err
			}
			responseData, responseDataBytes, err := validateAndVerifyEncodedRequest(attestationRequest, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.RawAndEncodedTeeAvailabilityResponseBody{
				ResponseData: types.TeeToExternal(responseData),
				ResponseBody: utils.BytesToHex0x(responseDataBytes),
			}), nil
		})
	// verify
	huma.Register(api, huma.Operation{
		OperationID:      "post-verify",
		Method:           http.MethodPost,
		Path:             getVerifierAPIPath(config.SourceIdPair.SourceId, config.AttestationTypePair.AttestationType, "verify"),
		Tags:             getVerifierAPITag(config.AttestationTypePair.AttestationType),
		SkipValidateBody: true, // TODO Check whether we can avoid this (here because huma changes bytes[32] to string)
	},
		func(ctx context.Context, request *struct {
			Body connector.IFtdcHubFtdcAttestationRequest
		}) (*types.Response[types.EncodedResponseBody], error) {
			logger.Debug("Received request for TEEAvailability")
			response, responseDataBytes, err := validateAndVerifyEncodedRequest(request.Body, ctx, config, verifier)
			if err != nil {
				logger.Error("Failed verifying request", err)
				return nil, err
			}
			logger.Debugf("Result of TEEAvailability verification: Status=%d, Timestamp=%d, CodeHash=%x, Platform=%s, InitialSigningPolicyId:%d, LastSigningPolicyId=%d, State=%v",
				response.Status,
				response.TeeTimestamp,
				response.CodeHash,
				bytes.Trim(response.Platform[:], "\x00"),
				response.InitialSigningPolicyId,
				response.LastSigningPolicyId,
				response.State)
			return types.NewResponse(types.EncodedResponseBody{
				Response: responseDataBytes,
			}), nil
		})
	// helper poller function
	huma.Register(api, huma.Operation{
		OperationID: "get-polled-tees",
		Method:      http.MethodGet,
		Path:        "/poller/tees",
		Tags:        []string{"Poller"},
	},
		func(ctx context.Context, req *struct{}) (*types.Response[types.TeeSamplesResponse], error) {
			teeVerifier, ok := verifier.(*teeverifier.TeeVerifier)
			if !ok {
				return nil, huma.NewError(
					http.StatusInternalServerError,
					"Internal server error: invalid verifier type",
				)
			}
			teeVerifier.SamplesMu.RLock()
			defer teeVerifier.SamplesMu.RUnlock()

			samples := make([]types.TeeSample, 0, len(teeVerifier.TeeSamples))
			for teeID, values := range teeVerifier.TeeSamples {
				samples = append(samples, types.TeeSample{
					TeeID:  teeID.Hex(),
					Values: values,
				})
			}
			return types.NewResponse(types.TeeSamplesResponse{
				Samples: samples,
			}), nil
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
