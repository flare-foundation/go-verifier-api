package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	utils "github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	config "github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func PMWPaymentStatusHandler(api huma.API, config *config.PMWPaymentStatusConfig, verifier verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody]) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/prepareRequestBody", config.AttestationTypePair.AttestationType),
		Tags:        []string{string(config.AttestationTypePair.AttestationType)}},
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
		Path:        fmt.Sprintf("/%s/prepareResponseBody", config.AttestationTypePair.AttestationType),
		Tags:        []string{string(config.AttestationTypePair.AttestationType)}},
		func(ctx context.Context, request *struct {
			Body types.PMWPaymentStatusEncodedRequest
		}) (*types.Response[types.RawAndEncodedPMWPaymentStatusResponseBody], error) {
			responseData, responseDataBytes, err := validateAndVerifyEncodedPMWPaymentStatusRequest(request.Body, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.RawAndEncodedPMWPaymentStatusResponseBody{
				ResponseData: responseData,
				ResponseBody: utils.HexWith0x(responseDataBytes),
			}), nil
		})
	// verify
	huma.Register(api, huma.Operation{
		OperationID: "post-verify",
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/verify", config.AttestationTypePair.AttestationType),
		Tags:        []string{string(config.AttestationTypePair.AttestationType)}},
		func(ctx context.Context, request *struct {
			Body types.PMWPaymentStatusEncodedRequest
		}) (*types.Response[types.EncodedResponseBody], error) {
			_, responseDataBytes, err := validateAndVerifyEncodedPMWPaymentStatusRequest(request.Body, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.EncodedResponseBody{
				ResponseBody: utils.HexWith0x(responseDataBytes),
			}), nil
		})
}

func validateAndVerifyEncodedPMWPaymentStatusRequest(request types.PMWPaymentStatusEncodedRequest, ctx context.Context, config *config.PMWPaymentStatusConfig, verifier verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody]) (connector.IPMWPaymentStatusResponseBody, []byte, error) {
	if err := validation.ValidateRequest(request); err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(config.AttestationTypePair, config.SourcePair, request.FTDCHeader.AttestationType, request.FTDCHeader.SourceId); err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
	}
	cleanRequestBodyHex := strings.TrimPrefix(request.RequestBody, "0x")
	requestBodyBytes, err := hex.DecodeString(cleanRequestBodyHex)
	if err != nil {
		return connector.IPMWPaymentStatusResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to bytes failed: %v", err))
	}
	requestData, err := utils.AbiDecodeRequestData[connector.IPMWPaymentStatusRequestBody](requestBodyBytes, config.AbiPair.Request)
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
