package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

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
			reqID := generateRequestID()
			err := validateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				return nil, warnHuma400(reqID, "Request validation failed", err)
			}
			encodedRequest, err := prepareRequestBody(request.Body, config)
			if err != nil {
				return nil, warnHuma400(reqID, "Prepare request failed", err)
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
			reqID := generateRequestID()
			err := validateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				return nil, warnHuma400(reqID, "Request validation failed", err)
			}
			requestData, err := decodeRequest[S](request.Body.RequestBody, config)
			if err != nil {
				return nil, warnHuma400(reqID, "Decoding request body to data failed", err)
			}
			responseData, err := verifier.Verify(ctx, requestData)
			if err != nil {
				return nil, classifyVerifyError(reqID, err)
			}
			encodedResponse, err := encodeResponse(responseData, config)
			if err != nil {
				return nil, warnHuma500(reqID, "Encoding data to response body failed", err)
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
			started := time.Now()
			reqID := generateRequestID()
			logger.Infof("[%s] Verify request started attestation=%s", reqID, string(attType))
			err := validateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex())
			if err != nil {
				return nil, warnHuma400(reqID, "Request validation failed", err)
			}
			requestData, err := decodeRequest[S](request.Body.RequestBody, config)
			if err != nil {
				return nil, warnHuma400(reqID, "Decoding request body to data failed", err)
			}
			logRequestBody(requestData)
			responseData, err := verifier.Verify(ctx, requestData)
			if err != nil {
				logger.Warnf("[%s] Verify request failed attestation=%s duration_ms=%d: %v",
					reqID, string(attType), time.Since(started).Milliseconds(), err)
				return nil, classifyVerifyError(reqID, err)
			}
			encodedResponse, err := encodeResponse(responseData, config)
			if err != nil {
				return nil, warnHuma500(reqID, "Encoding data to response body failed", err)
			}
			var v V
			responseDataExternal := v.FromInternal(responseData)
			responseDataExternal.Log()
			logger.Infof("[%s] Verify request finished attestation=%s status=success duration_ms=%d",
				reqID, string(attType), time.Since(started).Milliseconds())

			return types.NewResponse(types.AttestationResponse{
				ResponseBody: encodedResponse,
			}), nil
		})
}

func classifyVerifyError(reqID string, err error) error {
	msg := "Verification failed"
	switch {
	// 400 — bad request
	case errors.Is(err, feeproofxrp.ErrNonceRangeTooLarge):
		return warnHuma400(reqID, msg, err)
	// 422 — data/validation errors
	case errors.Is(err, feeproofxrp.ErrMissingPayEvent),
		errors.Is(err, feeproofxrp.ErrMissingTransaction),
		errors.Is(err, client.ErrRPCNonSuccess),
		errors.Is(err, db.ErrRecordNotFound),
		errors.Is(err, verifier.ErrTEEDataValidation),
		errors.Is(err, verifiertypes.ErrInvalidInput):
		return warnHuma422(reqID, msg, err)
	// 503 — infrastructure errors (retry)
	case errors.Is(err, client.ErrGetAccountInfo),
		errors.Is(err, db.ErrDatabase),
		errors.Is(err, verifier.ErrInsufficientSamples),
		errors.Is(err, verifiertypes.ErrNetwork),
		errors.Is(err, verifiertypes.ErrRPC),
		errors.Is(err, verifiertypes.ErrContext),
		errors.Is(err, verifiertypes.ErrUnknown),
		errors.Is(err, fetcher.ErrHTTPFetch),
		errors.Is(err, verifier.ErrActionResultNotFound):
		return warnHuma503(reqID, msg, err)
	// 500 — unexpected/ambiguous errors
	default:
		return warnHuma500(reqID, msg, err)
	}
}

var reqIDCounter uint64

func generateRequestID() string {
	return fmt.Sprintf("%08x", atomic.AddUint64(&reqIDCounter, 1))
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
