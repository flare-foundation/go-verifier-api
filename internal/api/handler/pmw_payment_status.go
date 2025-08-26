package handler

import (
	"context"
	"encoding/hex"
	"fmt"
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
	config *config.PMWPaymentStatusConfig,
	verifier verifierinterface.VerifierInterface[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody]) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        types.GetVerifierAPIPath(config.SourcePair.SourceId, config.AttestationTypePair.AttestationType, "prepareRequestBody"),
		Tags:        types.GetVerifierAPITag(config.AttestationTypePair.AttestationType)},
		func(ctx context.Context, request *struct {
			Body types.PMWPaymentStatusRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validation.ValidateRequest(request); err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
			}
			if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(config.AttestationTypePair, config.SourcePair, request.Body.FTDCHeader.AttestationType, request.Body.FTDCHeader.SourceId); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
			}
			requestDataInternal := request.Body.RequestData.ToInternal()
			// TODO-later add validation (later, now just use it as a helper to generate abi encoded request)
			requestDataBytes, err := utils.AbiEncodeData[connector.IPMWPaymentStatusRequestBody](requestDataInternal, config.AbiPair.Request)
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Encoding request data failed: %v", err))
			}
			return types.NewResponse(types.EncodedRequestBody{
				RequestBody: utils.HexWith0x(requestDataBytes),
			}), nil
		})
	// prepare ResponseBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareResponseBody",
		Method:      http.MethodPost,
		Path:        types.GetVerifierAPIPath(config.SourcePair.SourceId, config.AttestationTypePair.AttestationType, "prepareResponseBody"),
		Tags:        types.GetVerifierAPITag(config.AttestationTypePair.AttestationType)},
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
				ResponseBody: utils.HexWith0x(responseDataBytes),
			}), nil
		})
	// verify
	huma.Register(api, huma.Operation{
		OperationID: "post-verify",
		Method:      http.MethodPost,
		Path:        types.GetVerifierAPIPath(config.SourcePair.SourceId, config.AttestationTypePair.AttestationType, "verify"),
		Tags:        types.GetVerifierAPITag(config.AttestationTypePair.AttestationType)},
		func(ctx context.Context, request *struct {
			Body types.FTDCRequestEncoded
		}) (*types.Response[types.EncodedResponseBody], error) {
			attestationRequest, err := toIFTdcHubFtdcAttestationRequest(request.Body)
			if err != nil {
				return nil, err
			}
			_, responseDataBytes, err := validateAndVerifyEncodedPMWPaymentStatusRequest(attestationRequest, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.EncodedResponseBody{
				Response: responseDataBytes,
			}), nil
		})
}

func validateAndVerifyEncodedPMWPaymentStatusRequest(request connector.IFtdcHubFtdcAttestationRequest, ctx context.Context, config *config.PMWPaymentStatusConfig, verifier verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody]) (connector.IPMWPaymentStatusResponseBody, []byte, error) {
	if err := validation.ValidateRequest(request); err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(
		config.AttestationTypePair,
		config.SourcePair,
		fmt.Sprintf("0x%s", hex.EncodeToString(request.Header.AttestationType[:])),
		fmt.Sprintf("0x%s", hex.EncodeToString(request.Header.SourceId[:])),
	); err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
	}
	requestData, err := utils.AbiDecodeRequestData[connector.IPMWPaymentStatusRequestBody](request.RequestBody, config.AbiPair.Request)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	responseData, err := verifier.Verify(ctx, requestData)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", err))
	}
	responseDataBytes, err := utils.AbiEncodeData[connector.IPMWPaymentStatusResponseBody](responseData, config.AbiPair.Response)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", err))
	}
	return responseData, responseDataBytes, nil
}
