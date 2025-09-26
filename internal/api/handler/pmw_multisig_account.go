package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func PMWMultisigAccountHandler(
	api huma.API,
	config *config.EncodedAndABI,
	verifier verifierinterface.VerifierInterface[
		connector.IPMWMultisigAccountConfiguredRequestBody,
		connector.IPMWMultisigAccountConfiguredResponseBody]) {
	srcID := config.SourceIDPair.SourceID
	attType := config.AttestationTypePair.AttestationType
	tags := getVerifierAPITag(attType)

	RegisterOp(api,
		"post-prepareRequestBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareRequestBody"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequestData[types.PMWMultisigAccountConfiguredRequestBody]
		}) (*types.Response[types.AttestationRequestEncoded], error) {
			err := ValidateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				logger.Errorf("Request validation failed: %v", err)
				return nil, huma.Error400BadRequest("Request validation failed: " + err.Error())
			}
			encodedRequest, err := PrepareRequestBody(request.Body, config)
			if err != nil {
				logger.Errorf("Prepare Request failed:", err)
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
		}) (*types.Response[types.AttestationResponseData[types.PMWMultisigAccountConfiguredResponseBody]], error) {
			err := ValidateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				logger.Errorf("Request validation failed: %v", err)
				return nil, huma.Error400BadRequest("Request validation failed: " + err.Error())
			}
			requestData, err := DecodeRequest[connector.IPMWMultisigAccountConfiguredRequestBody](request.Body.RequestBody, config)
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
			return types.NewResponse(types.AttestationResponseData[types.PMWMultisigAccountConfiguredResponseBody]{
				ResponseData: types.PMWMultisigAccountResponseToExternal(responseData),
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
			logger.Debug("Received request for PMWMultisigAccountConfigured")
			err := ValidateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				logger.Errorf("Request validation failed: %v", err)
				return nil, huma.Error400BadRequest("Request validation failed: " + err.Error())
			}
			requestData, err := DecodeRequest[connector.IPMWMultisigAccountConfiguredRequestBody](request.Body.RequestBody, config)
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
			logPMWMultisigAccountResponse(responseData)
			return types.NewResponse(types.AttestationResponse{
				ResponseBody: encodedResponse,
			}), nil
		})
}
