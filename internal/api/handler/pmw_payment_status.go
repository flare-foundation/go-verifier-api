package handler

import (
	"context"
	"fmt"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func PMWPaymentStatusHandler(
	api huma.API,
	config *config.EncodedAndAbi,
	verifier verifierinterface.VerifierInterface[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody]) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        getVerifierAPIPath(config.SourceIdPair.SourceId, config.AttestationTypePair.AttestationType, "prepareRequestBody"),
		Tags:        getVerifierAPITag(config.AttestationTypePair.AttestationType)},
		func(ctx context.Context, request *struct {
			Body types.PMWPaymentStatusRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validation.ValidateRequest(request); err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
			}
			if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(config.AttestationTypePair, config.SourceIdPair, request.Body.FTDCHeader.AttestationType, request.Body.FTDCHeader.SourceId); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
			}
			requestDataInternal, err := request.Body.RequestData.ToInternal()
			// TODO-later add validation (later, now just use it as a helper to generate abi encoded request)
			requestDataBytes, err := utils.AbiEncodeData[connector.IPMWPaymentStatusRequestBody](requestDataInternal, config.AbiPair.Request)
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
		}) (*types.Response[types.RawAndEncodedPMWPaymentStatusResponseBody], error) {
			attestationRequest, err := toIFTdcHubFtdcAttestationRequest(request.Body)
			if err != nil {
				return nil, err
			}
			responseData, responseDataBytes, err := validateAndVerifyEncodedPMWPaymentStatusRequest(attestationRequest, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.RawAndEncodedPMWPaymentStatusResponseBody{
				ResponseData: types.PMWPaymentToExternal(responseData),
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
			logger.Debug("Received request for PMWPaymentStatusRequest")
			_, responseDataBytes, err := validateAndVerifyEncodedPMWPaymentStatusRequest(request.Body, ctx, config, verifier)
			logger.Debug("Received request for PMWPaymentStatusRequest")
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.EncodedResponseBody{
				Response: responseDataBytes,
			}), nil
		})
}

func validateAndVerifyEncodedPMWPaymentStatusRequest(request connector.IFtdcHubFtdcAttestationRequest, ctx context.Context, config *config.EncodedAndAbi, verifier verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody]) (connector.IPMWPaymentStatusResponseBody, []byte, error) {
	requestData, err := validateAndParseFTDCRequest[connector.IPMWPaymentStatusRequestBody](request, config)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, err
	}
	responseData, err := verifier.Verify(ctx, requestData)
	return handleVerifierResult[connector.IPMWPaymentStatusResponseBody](err, responseData, config)
}
