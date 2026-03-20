package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	feeproofxrp "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_fee_proof/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/fetcher"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func RegisterVerificationHandler[S, T any, U types.RequestConvertible[S], V types.ResponseConvertible[T]](
	api huma.API,
	config *config.EncodedAndABI,
	verifier attestation.Verifier[S, T],
) {
	srcID := config.SourceIDPair.SourceID
	attType := config.AttestationTypePair.AttestationType
	tags := getVerifierAPITag(attType)

	registerOp(api,
		"post-prepareRequestBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareRequestBody"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequestData[U]
		}) (*types.Response[types.AttestationRequestEncoded], error) {
			err := validateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				return nil, warnHuma400("Request validation failed", err)
			}
			encodedRequest, err := prepareRequestBody(request.Body, config)
			if err != nil {
				return nil, warnHuma400("Prepare request failed", err)
			}
			return types.NewResponse(types.AttestationRequestEncoded{
				RequestBody: encodedRequest,
			}), nil
		})

	registerOp(api,
		"post-prepareResponseBody",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "prepareResponseBody"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequest
		}) (*types.Response[types.AttestationResponseData[types.ResponseConvertible[T]]], error) {
			err := validateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				return nil, warnHuma400("Request validation failed", err)
			}
			requestData, err := decodeRequest[S](request.Body.RequestBody, config)
			if err != nil {
				return nil, warnHuma400("Decoding request body to data failed", err)
			}
			responseData, err := verifier.Verify(ctx, requestData)
			if err != nil {
				return nil, classifyVerifyError(err)
			}
			encodedResponse, err := encodeResponse(responseData, config)
			if err != nil {
				return nil, warnHuma500("Encoding data to response body failed", err)
			}

			var v V
			responseDataExternal := v.FromInternal(responseData)
			attestationResponse := types.AttestationResponseData[types.ResponseConvertible[T]]{
				ResponseData: responseDataExternal,
				ResponseBody: encodedResponse,
			}

			return &types.Response[types.AttestationResponseData[types.ResponseConvertible[T]]]{Body: attestationResponse}, nil
		})

	registerOp(api,
		"post-verify",
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "verify"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequest
		}) (*types.Response[types.AttestationResponse], error) {
			logger.Debugf("Received request for %s", string(attType))
			err := validateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				return nil, warnHuma400("Request validation failed", err)
			}
			requestData, err := decodeRequest[S](request.Body.RequestBody, config)
			if err != nil {
				return nil, warnHuma400("Decoding request body to data failed", err)
			}
			logRequestBody(requestData)
			responseData, err := verifier.Verify(ctx, requestData)
			if err != nil {
				return nil, classifyVerifyError(err)
			}
			encodedResponse, err := encodeResponse(responseData, config)
			if err != nil {
				return nil, warnHuma500("Encoding data to response body failed", err)
			}
			var v V
			responseDataExternal := v.FromInternal(responseData)
			responseDataExternal.Log()

			return types.NewResponse(types.AttestationResponse{
				ResponseBody: encodedResponse,
			}), nil
		})
}

func classifyVerifyError(err error) error {
	switch {
	// 400 — bad request
	case errors.Is(err, feeproofxrp.ErrNonceRangeTooLarge):
		return warnHuma400("Verification failed", err)
	// 422 — data/validation errors
	case errors.Is(err, feeproofxrp.ErrMissingPayEvent):
		return warnHuma422("Verification failed", err)
	case errors.Is(err, feeproofxrp.ErrMissingTransaction):
		return warnHuma422("Verification failed", err)
	case errors.Is(err, client.ErrRPCNonSuccess):
		return warnHuma422("Verification failed", err)
	case errors.Is(err, db.ErrRecordNotFound):
		return warnHuma422("Verification failed", err)
	case errors.Is(err, verifier.ErrTEEDataValidation):
		return warnHuma422("Verification failed", err)
	case errors.Is(err, verifiertypes.ErrInvalidInput):
		return warnHuma422("Verification failed", err)
	// 503 — infrastructure errors (retry)
	case errors.Is(err, client.ErrGetAccountInfo):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, db.ErrDatabase):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, verifier.ErrInsufficientSamples):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, verifiertypes.ErrNetwork):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, verifiertypes.ErrRPC):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, verifiertypes.ErrContext):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, verifiertypes.ErrUnknown):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, fetcher.ErrHTTPFetch):
		return warnHuma503("Verification failed", err)
	case errors.Is(err, verifier.ErrActionResultNotFound):
		return warnHuma503("Verification failed", err)
	// 500 — unexpected/ambiguous errors
	default:
		return warnHuma500("Verification failed", err)
	}
}

func logRequestBody[T any](requestData T) {
	switch req := any(requestData).(type) {
	case connector.ITeeAvailabilityCheckRequestBody:
		types.LogTeeAvailabilityCheckRequestBody(req)
	case connector.IPMWMultisigAccountConfiguredRequestBody:
		types.LogPMWMultisigAccountConfiguredRequestBody(req)
	case connector.IPMWPaymentStatusRequestBody:
		types.LogPMWPaymentStatusRequestBody(req)
	case connector.IPMWFeeProofRequestBody:
		types.LogPMWFeeProofRequestBody(req)
	default:
		logger.Debug("No request logger for this request type")
	}
}
