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

func PMWPaymentStatusHandler(
	api huma.API,
	config *config.EncodedAndAbi,
	verifier verifierinterface.VerifierInterface[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody]) {
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
			Body types.PMWPaymentStatusRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validatePrepareResponseBody[types.PMWPaymentStatusRequestBody](request.Body, config); err != nil {
				return nil, err
			}
			requestData, err := request.Body.RequestData.ToInternal()
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
			}
			return prepareRequestBody[connector.IPMWPaymentStatusRequestBody](requestData, config)
		})

	RegisterOp(api,
		"post-prepareResponseBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareResponseBody"),
		tags,
		false,
		func(ctx context.Context, request *struct {
			Body types.FTDCRequestEncoded
		}) (*types.Response[types.RawAndEncodedPMWPaymentStatusResponseBody], error) {
			return prepareResponseBody(
				ctx,
				request.Body,
				validateAndVerifyEncodedPMWPaymentStatusRequest,
				types.PMWPaymentToExternal,
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
			logger.Debug("Received request for PMWPaymentStatusRequest (verify)")
			responseData, responseDataBytes, err := validateAndVerifyEncodedPMWPaymentStatusRequest(request.Body, ctx, config, verifier)
			if err != nil {
				logger.Error("Failed verifying request", err)
				return nil, err
			}
			logPMWPaymentStatusResponse(responseData)
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
