package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func PMWMultisigAccountHandler(api huma.API, config *config.PMWMultisigAccountConfig, verifier verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody]) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        attestationtypes.GetVerifierPath(config.SourcePair.SourceId, config.AttestationTypePair.AttestationType, "prepareRequestBody"),
		Tags:        []string{string(config.AttestationTypePair.AttestationType)}},
		func(ctx context.Context, request *struct {
			Body types.PMWMultisigAccountRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validation.ValidateRequest(request); err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
			}
			if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(config.AttestationTypePair, config.SourcePair, request.Body.FTDCHeader.AttestationType, request.Body.FTDCHeader.SourceId); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
			}
			requestData, err := request.Body.RequestData.ToInternal()
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
			}
			// TODO-later add validation (later, now just use it as a helper to generate abi encoded request)
			requestDataBytes, err := utils.AbiEncodeData[connector.IPMWMultisigAccountConfiguredRequestBody](requestData, config.AbiPair.Request)
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
		Path:        attestationtypes.GetVerifierPath(config.SourcePair.SourceId, config.AttestationTypePair.AttestationType, "prepareResponseBody"),
		Tags:        []string{string(config.AttestationTypePair.AttestationType)}},
		func(ctx context.Context, request *struct {
			Body types.FTDCRequestEncoded
		}) (*types.Response[types.RawAndEncodedPMWMultisigAccountResponseBody], error) {
			attestationRequest, err := toIFTdcHubFtdcAttestationRequest(request.Body)
			if err != nil {
				return nil, err
			}
			responseData, responseDataBytes, err := validateAndVerifyEncodedPMWMultisigAccountRequest(attestationRequest, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.RawAndEncodedPMWMultisigAccountResponseBody{
				ResponseData: types.MultiSigToExternal(responseData),
				ResponseBody: utils.HexWith0x(responseDataBytes),
			}), nil
		})
	// verify
	huma.Register(api, huma.Operation{
		OperationID:      "post-verify",
		Method:           http.MethodPost,
		Path:             attestationtypes.GetVerifierPath(config.SourcePair.SourceId, config.AttestationTypePair.AttestationType, "verify"),
		Tags:             []string{string(config.AttestationTypePair.AttestationType)},
		SkipValidateBody: true, // TODO Check whether we can avoid this (here because huma changes bytes[32] to string)
	},

		func(ctx context.Context, request *struct {
			Body connector.IFtdcHubFtdcAttestationRequest
		}) (*types.Response[types.EncodedResponseBody], error) {
			logger.Debug("Received request for PMWMultisigAccount")
			responseData, responseDataBytes, err := validateAndVerifyEncodedPMWMultisigAccountRequest(request.Body, ctx, config, verifier)
			if err != nil {
				logger.Error("Failed verifying request", err)
				return nil, err
			}
			logger.Debug("Result after PMWMultisigAccount verification", responseData)
			return types.NewResponse(types.EncodedResponseBody{
				Response: responseDataBytes,
			}), nil
		})
}

func validateAndVerifyEncodedPMWMultisigAccountRequest(request connector.IFtdcHubFtdcAttestationRequest, ctx context.Context, config *config.PMWMultisigAccountConfig, verifier verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody]) (connector.IPMWMultisigAccountConfiguredResponseBody, []byte, error) {
	if err := validation.ValidateRequest(request); err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(
		config.AttestationTypePair,
		config.SourcePair,
		fmt.Sprintf("0x%s", hex.EncodeToString(request.Header.AttestationType[:])),
		fmt.Sprintf("0x%s", hex.EncodeToString(request.Header.SourceId[:])),
	); err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
	}
	requestData, err := utils.AbiDecodeRequestData[connector.IPMWMultisigAccountConfiguredRequestBody](request.RequestBody, config.AbiPair.Request)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	responseData, err := verifier.Verify(ctx, requestData)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", err))
	}
	responseDataBytes, err := utils.AbiEncodeData[connector.IPMWMultisigAccountConfiguredResponseBody](responseData, config.AbiPair.Response)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", err))
	}
	return responseData, responseDataBytes, nil
}
