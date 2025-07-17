package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/go-playground/validator/v10"
	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/types"
	attestationutils "gitlab.com/urskak/verifier-api/internal/api/utils"
	"gitlab.com/urskak/verifier-api/internal/api/validation"
	teecrypto "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/crypto"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("hash32", validation.IsHash32)
	validate.RegisterValidation("eth_addr", validation.IsCommonAddress)
}

// Response is a generic response type for the API with just a simple body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
type Response[T any] struct {
	Body T
}

// NewResponse returns the response type with the right body. https://zuplo.com/blog/2025/04/20/how-to-build-an-api-with-go-and-huma
func NewResponse[T any](body T) *Response[T] {
	return &Response[T]{Body: body}
}

func TeeAvailabilityCheckHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[attestationtypes.ITeeAvailabilityCheckRequestBody, attestationtypes.ITeeAvailabilityCheckResponseBody], sourceID string) {
	huma.Post(api, fmt.Sprintf("/%s/%s", attestationType, "prepareRequestBody"), func(ctx context.Context, request *struct {
		Body attestationtypes.IFtdcHubFtdcRequestHeaderTeeAvailabilityCheck
	}) (*Response[attestationtypes.EncodedRequestBody], error) {
		if err := validateRequest(request); err != nil {
			return nil, err
		}
		if err := validateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		res, err := teecrypto.AbiEncodeRequestBody(request.Body.RequestBody)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("encoding failed: %v", err))
		}
		// TODO verify
		return NewResponse(attestationtypes.EncodedRequestBody{
			EncodedRequestBody: attestationutils.HexWith0x(res),
		}), nil
	})

	huma.Post(api, fmt.Sprintf("/%s/%s", attestationType, "prepareResponseBody"), func(ctx context.Context, request *struct {
		Body attestationtypes.IFtdcHubFtdcRequestHeaderTeeAvailabilityCheck
	}) (*Response[attestationtypes.EncodedResponseBody], error) {
		if err := validateRequest(request); err != nil {
			return nil, err
		}
		if err := validateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		// TODO verify
		// TODO prepare encoded and decoded response body
		return nil, huma.Error501NotImplemented("TeeAvailabilityChecky - prepareResponseBody")
	})

	huma.Post(api, fmt.Sprintf("/%s/%s", attestationType, "verify"), func(ctx context.Context, request *struct {
		Body attestationtypes.IFtdcHubFtdcRequestHeaderTeeAvailabilityCheckEncoded
	}) (*Response[attestationtypes.EncodedResponseBody], error) {
		if err := validateRequest(request); err != nil {
			return nil, err
		}
		if err := validateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		cleanRequestBodyHex := strings.TrimPrefix(request.Body.RequestBody, "0x")
		requestBodyBytes, err := hex.DecodeString(cleanRequestBodyHex)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("%v", err))
		}
		requestBody, err := teecrypto.AbiDecodeRequestBody(requestBodyBytes)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("decoding failed: %v", err))
		}
		_, err = verifier.Verify(ctx, requestBody)
		if err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("verification failed: %v", err))
		}
		//TODO - encode actual response
		response := attestationtypes.EncodedResponseBody{EncodedResponseBody: attestationutils.HexWith0x([]byte{})}
		return NewResponse(response), nil
	})
}

func PMWPaymentStatusHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[attestationtypes.IPMWPaymentStatusRequestBody, attestationtypes.IPMWPaymentStatusResponseBody], sourceID string) {
	huma.Error501NotImplemented("PMW payment status not implemented yet")
}

func validateRequest(request interface{}) error {
	if err := validate.Struct(request); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("validation failed: %v", err))
	}
	return nil
}

func validateSystemAndRequestAttestationNameAndSourceId(systemAttestationType connector.AttestationType, systemSourceId string, requestAttestationName string, requestSourceId string) error {
	verifierAttestationNameEnc, err := encodeAttestationOrSourceName(string(systemAttestationType))
	if err != nil {
		return huma.Error500InternalServerError(fmt.Sprintf("system attestation type name encoding failed: %v", err))
	}
	verifierSourceNameEnc, err := encodeAttestationOrSourceName(systemSourceId)
	if err != nil {
		return huma.Error500InternalServerError(fmt.Sprintf("system source name encoding failed: %v", err))
	}
	if requestAttestationName != verifierAttestationNameEnc || string(requestSourceId) != verifierSourceNameEnc {
		return huma.Error400BadRequest(fmt.Sprintf(
			"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s).",
			requestAttestationName, requestSourceId,
			string(systemAttestationType), verifierAttestationNameEnc,
			systemSourceId, verifierSourceNameEnc,
		))
	}
	return nil
}

func encodeAttestationOrSourceName(attestationTypeOrSourceName string) (string, error) {
	if len(attestationTypeOrSourceName) >= 2 && (attestationTypeOrSourceName[:2] == "0x" || attestationTypeOrSourceName[:2] == "0X") {
		return "", fmt.Errorf("attestation type or source id name must not start with '0x'. Provided: '%s'", attestationTypeOrSourceName)
	}
	bytes := []byte(attestationTypeOrSourceName)
	if len(bytes) > 32 {
		return "", fmt.Errorf("attestation type or source id name '%s' is too long (%d bytes)", attestationTypeOrSourceName, len(bytes))
	}
	padded := make([]byte, 32)
	copy(padded, bytes)
	return "0x" + hex.EncodeToString(padded), nil
}
