package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func PMWMultisigAccountHandler(
	api huma.API,
	config *config.EncodedAndAbi,
	verifier verifierinterface.VerifierInterface[
		connector.IPMWMultisigAccountConfiguredRequestBody,
		connector.IPMWMultisigAccountConfiguredResponseBody]) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        getVerifierAPIPath(config.SourceIdPair.SourceId, config.AttestationTypePair.AttestationType, "prepareRequestBody"),
		Tags:        getVerifierAPITag(config.AttestationTypePair.AttestationType)},
		func(ctx context.Context, request *struct {
			Body types.PMWMultisigAccountRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validatePrepareResponseBody[types.PMWMultisigAccountRequestBody](request.Body, config); err != nil {
				return nil, err
			}
			requestData, err := request.Body.RequestData.ToInternal()
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
			}
			return prepareRequestBody[connector.IPMWMultisigAccountConfiguredRequestBody](requestData, config)
		})
	// prepare ResponseBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareResponseBody",
		Method:      http.MethodPost,
		Path:        getVerifierAPIPath(config.SourceIdPair.SourceId, config.AttestationTypePair.AttestationType, "prepareResponseBody"),
		Tags:        getVerifierAPITag(config.AttestationTypePair.AttestationType)},
		func(ctx context.Context, request *struct {
			Body types.FTDCRequestEncoded
		}) (*types.Response[types.RawAndEncodedPMWMultisigAccountResponseBody], error) {
			return prepareResponseBody(
				ctx,
				request.Body,
				validateAndVerifyEncodedPMWMultisigAccountRequest,
				types.MultiSigToExternal,
				config,
				verifier,
			)
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
			logger.Debug("Received request for PMWMultisigAccount (verify)")
			responseData, responseDataBytes, err := validateAndVerifyEncodedPMWMultisigAccountRequest(request.Body, ctx, config, verifier)
			if err != nil {
				logger.Error("Failed verifying request", err)
				return nil, err
			}
			logPMWMultisigAccountResponse(responseData)
			return types.NewResponse(types.EncodedResponseBody{
				Response: responseDataBytes,
			}), nil
		})
}

func validateAndVerifyEncodedPMWMultisigAccountRequest(request connector.IFtdcHubFtdcAttestationRequest, ctx context.Context, config *config.EncodedAndAbi, verifier verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody]) (connector.IPMWMultisigAccountConfiguredResponseBody, []byte, error) {
	requestData, err := validateAndParseFTDCRequest[connector.IPMWMultisigAccountConfiguredRequestBody](request, config)
	if err != nil {
		return connector.IPMWMultisigAccountConfiguredResponseBody{}, []byte{}, err
	}
	logger.Debugf("Verify PMWMultisigAccount for %s, threshold %d", requestData.WalletAddress, requestData.Threshold)
	responseData, err := verifier.Verify(ctx, requestData)
	return handleVerifierResult[connector.IPMWMultisigAccountConfiguredResponseBody](err, responseData, config)
}
