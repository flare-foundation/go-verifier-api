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
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	feeproofxrp "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/xrp"
	multisigxrp "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/fetcher"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// verifierWorkTimeout is the authoritative deadline on a single verification's
// work (DB queries + Flare RPC). It is kept below the server writeTimeout so the
// verifier abandons a hung dependency and returns before the HTTP write deadline
// fires. Downstream calls run under this context, so the deadline actually
// cancels them: the DB repos use WithContext and the nonce binder uses the ctx.
const verifierWorkTimeout = 25 * time.Second

// verifyWithDeadline runs the verifier under an authoritative timeout so a slow or
// hung dependency cannot pin the request goroutine indefinitely. A timed-out
// verification surfaces context.DeadlineExceeded, classified as 503.
func verifyWithDeadline[S, T any](ctx context.Context, v attestation.Verifier[S, T], req S, timeout time.Duration) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return v.Verify(ctx, req)
}

func RegisterVerificationHandler[S, T any, U types.RequestConvertible[S], V types.ResponseConvertible[T]](
	api huma.API,
	config *config.EncodedAndABI,
	verifier attestation.Verifier[S, T],
) {
	srcID := config.SourceIDPair.SourceID
	attType := config.AttestationTypePair.AttestationType
	tags := getVerifierAPITag(attType)

	registerOp(api,
		getVerifierOperationID(srcID, attType, "prepareRequestBody"),
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
		getVerifierOperationID(srcID, attType, "prepareResponseBody"),
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
			responseData, err := verifyWithDeadline(ctx, verifier, requestData, verifierWorkTimeout)
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

	// post-verify uses the status-based envelope contract (types.VerifierResponse)
	// consumed by tee-relay-client: every verification outcome is returned as HTTP
	// 200 with a {status, responseBody, message} body. The relay decodes the body
	// only on 2xx and switches on status (VERIFIED / RETRY / REJECTED); a non-2xx
	// is treated as a transport failure and retried, so genuinely transient
	// infrastructure errors are reported in-band as RETRY, not as an HTTP error.
	registerOp(api,
		getVerifierOperationID(srcID, attType, "verify"),
		http.MethodPost,
		getVerifierAPIPath(srcID, attType, "verify"),
		tags,
		func(ctx context.Context, request *struct {
			Body types.AttestationRequest
		}) (*types.Response[types.VerifierResponse], error) {
			started := time.Now()
			reqID := generateRequestID()
			logger.Infof("[%s] Verify request started attestation=%s", reqID, string(attType))
			if err := validateSystemAndRequestAttestationNameAndSourceID(config, request.Body.AttestationType.Hex(), request.Body.SourceID.Hex()); err != nil {
				return rejectedResponse(reqID, "Request validation failed", "unsupported attestation type or source id", err), nil
			}
			requestData, err := decodeRequest[S](request.Body.RequestBody, config)
			if err != nil {
				return rejectedResponse(reqID, "Decoding request body to data failed", "malformed request body", err), nil
			}
			logRequestBody(requestData)
			responseData, err := verifyWithDeadline(ctx, verifier, requestData, verifierWorkTimeout)
			if err != nil {
				status, message := classifyVerifyStatus(err)
				logger.Warnf("[%s] Verify request failed attestation=%s status=%s duration_ms=%d: %v",
					reqID, string(attType), status, time.Since(started).Milliseconds(), err)
				return types.NewResponse(types.VerifierResponse{Status: status, Message: message}), nil
			}
			encodedResponse, err := encodeResponse(responseData, config)
			if err != nil {
				return retryResponse(reqID, "Encoding data to response body failed", "response encoding failed", err), nil
			}
			var v V
			responseDataExternal := v.FromInternal(responseData)
			responseDataExternal.Log()
			logger.Infof("[%s] Verify request finished attestation=%s status=VERIFIED duration_ms=%d",
				reqID, string(attType), time.Since(started).Milliseconds())

			return types.NewResponse(types.VerifierResponse{
				Status:       types.StatusVerified,
				ResponseBody: encodedResponse,
			}), nil
		})
}

// classifyVerifyStatus maps a verification error to the /verify envelope's
// (status, safe message). It mirrors classifyVerifyError's error sets used by the
// HTTP-status prepare* endpoints: the deterministic classes (400/422 there) map
// to REJECTED (terminal), while the infrastructure class (503 there) and any
// unexpected error map to RETRY (retryable). The message is a coarse,
// non-sensitive category; internal error detail stays in the server log only.
func classifyVerifyStatus(err error) (status, message string) {
	switch {
	case errors.Is(err, feeproofxrp.ErrBatchRangeTooLarge):
		return types.StatusRejected, "batch range too large"
	case errors.Is(err, feeproofxrp.ErrReissueLimitExceeded):
		return types.StatusRejected, "reissue limit exceeded"
	case errors.Is(err, multisigxrp.ErrInvalidRequest):
		return types.StatusRejected, "invalid request"
	case errors.Is(err, feeproofxrp.ErrMissingPayEvent):
		return types.StatusRejected, "missing pay event for the payment"
	case errors.Is(err, feeproofxrp.ErrMissingTransaction):
		return types.StatusRejected, "missing transaction for the payment"
	case errors.Is(err, client.ErrRPCNonSuccess):
		return types.StatusRejected, "source reported a non-success result"
	case errors.Is(err, db.ErrRecordNotFound):
		return types.StatusRejected, "record not found"
	case errors.Is(err, verifier.ErrTEEChallengeMismatch):
		return types.StatusRejected, "TEE challenge mismatch"
	case errors.Is(err, verifier.ErrTEEChainIDMismatch):
		return types.StatusRejected, "TEE chain id mismatch"
	case errors.Is(err, verifier.ErrTEEProxySignerMismatch):
		return types.StatusRejected, "TEE proxy signer mismatch"
	case errors.Is(err, verifier.ErrTEESigningPolicyHash):
		return types.StatusRejected, "TEE signing policy hash mismatch"
	case errors.Is(err, verifier.ErrTEEAttestationInvalid):
		return types.StatusRejected, "TEE attestation invalid"
	case errors.Is(err, verifier.ErrTEEResponseMalformed):
		return types.StatusRejected, "TEE response malformed"
	case errors.Is(err, verifier.ErrTEEActionResultMismatch):
		return types.StatusRejected, "TEE action result mismatch"
	// Transient: a revocation-check (CRL) fetch failure must retry, not reject.
	// Checked before the generic ErrTEEDataValidation case (which it does NOT chain).
	case errors.Is(err, verifier.ErrTEERevocationUnavailable):
		return types.StatusRetry, "TEE revocation check unavailable"
	case errors.Is(err, verifier.ErrTEEDataValidation):
		return types.StatusRejected, "TEE data validation failed"
	case errors.Is(err, verifiertypes.ErrInvalidInput):
		return types.StatusRejected, "invalid input"
	case errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled):
		return types.StatusRetry, "verification timed out"
	case errors.Is(err, client.ErrFetchAccountInfo),
		errors.Is(err, client.ErrFetchServerInfo),
		errors.Is(err, client.ErrRPCTransient):
		return types.StatusRetry, "source RPC unavailable"
	case errors.Is(err, multisigxrp.ErrNetworkMismatch):
		return types.StatusRetry, "source network not verified"
	case errors.Is(err, db.ErrDatabase):
		return types.StatusRetry, "database unavailable"
	case errors.Is(err, db.ErrDataSource):
		return types.StatusRetry, "data source returned unusable data"
	case errors.Is(err, verifiertypes.ErrNetwork),
		errors.Is(err, verifiertypes.ErrRPC),
		errors.Is(err, verifiertypes.ErrContext),
		errors.Is(err, verifiertypes.ErrUnknown):
		return types.StatusRetry, "network or RPC error"
	case errors.Is(err, fetcher.ErrHTTPFetch):
		return types.StatusRetry, "upstream fetch failed"
	case errors.Is(err, verifier.ErrActionResultNotFound):
		return types.StatusRetry, "action result not available"
	default:
		return types.StatusRetry, "unexpected error"
	}
}

func classifyVerifyError(reqID string, err error) error {
	msg := "Verification failed"
	switch {
	// 400 — bad request
	case errors.Is(err, feeproofxrp.ErrBatchRangeTooLarge),
		errors.Is(err, feeproofxrp.ErrReissueLimitExceeded),
		errors.Is(err, multisigxrp.ErrInvalidRequest):
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
	case errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled),
		errors.Is(err, client.ErrFetchAccountInfo),
		errors.Is(err, client.ErrFetchServerInfo),
		errors.Is(err, client.ErrRPCTransient),
		errors.Is(err, multisigxrp.ErrNetworkMismatch),
		errors.Is(err, db.ErrDatabase),
		errors.Is(err, db.ErrDataSource),
		errors.Is(err, verifiertypes.ErrNetwork),
		errors.Is(err, verifiertypes.ErrRPC),
		errors.Is(err, verifiertypes.ErrContext),
		errors.Is(err, verifiertypes.ErrUnknown),
		errors.Is(err, fetcher.ErrHTTPFetch),
		errors.Is(err, verifier.ErrActionResultNotFound),
		errors.Is(err, verifier.ErrTEERevocationUnavailable):
		return warnHuma503(reqID, msg, err)
	// 500 — unexpected/ambiguous errors
	default:
		return warnHuma500(reqID, msg, err)
	}
}

var reqIDCounter atomic.Uint64

func generateRequestID() string {
	return fmt.Sprintf("%08x", reqIDCounter.Add(1))
}

func logRequestBody[T any](requestData T) {
	switch req := any(requestData).(type) {
	case fdc2.ITeeAvailabilityCheckRequestBody:
		types.LogTeeAvailabilityCheckRequestBody(req)
	case fdc2.IPMWMultisigAccountConfiguredRequestBody:
		types.LogPMWMultisigAccountConfiguredRequestBody(req)
	case fdc2.IPMWPaymentStatusRequestBody:
		types.LogPMWPaymentStatusRequestBody(req)
	case fdc2.IPMWFeeProofRequestBody:
		types.LogPMWFeeProofRequestBody(req)
	default:
		logger.Debug("No request logger for this request type")
	}
}
