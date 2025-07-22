package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "gitlab.com/urskak/verifier-api/internal/api/type"
	teecrypto "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/crypto"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

func TeeAvailabilityCheckHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], sourceID string) {
	// prepare RequestBody
	huma.Post(api, fmt.Sprintf("/%s/prepareRequestBody", attestationType), func(ctx context.Context, request *struct {
		Body types.TeeAvailabilityRequest
	}) (*types.Response[types.EncodedRequestBody], error) {
		if err := ValidateRequest(request); err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.Header.AttestationType, request.Body.Header.SourceId); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
		}
		requestData, err := request.Body.RequestBody.ToInternal()
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
		}
		// TODO validate
		requestDataBytes, err := teecrypto.AbiEncodeRequestData(requestData)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Encoding request data failed: %v", err))
		}
		return types.NewResponse(types.EncodedRequestBody{
			EncodedRequestBody: HexWith0x(requestDataBytes),
		}), nil
	})
	// prepare ResponseBody
	huma.Post(api, fmt.Sprintf("/%s/prepareResponseBody", attestationType), func(ctx context.Context, request *struct {
		Body types.TeeAvailabilityEncodedRequest
	}) (*types.Response[types.RawAndEncodedResponseBody], error) {
		if err := ValidateRequest(request); err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.Header.AttestationType, request.Body.Header.SourceId); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
		}
		cleanRequestBodyHex := strings.TrimPrefix(request.Body.RequestBody, "0x")
		requestBodyBytes, err := hex.DecodeString(cleanRequestBodyHex)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to bytes failed: %v", err))
		}
		requestData, err := teecrypto.AbiDecodeRequestData(requestBodyBytes)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
		}
		responseData, err := verifier.Verify(ctx, requestData)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", err))
		}
		responseDataBytes, err := teecrypto.AbiEncodeResponseData(responseData)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", err))
		}
		return types.NewResponse(types.RawAndEncodedResponseBody{
			ResponseBody:        responseData.ToExternal(),
			EncodedResponseBody: HexWith0x(responseDataBytes),
		}), nil
	})
	// verify
	huma.Post(api, fmt.Sprintf("/%s/verify", attestationType), func(ctx context.Context, request *struct {
		Body types.TeeAvailabilityEncodedRequest
	}) (*types.Response[types.EncodedResponseBody], error) {
		if err := ValidateRequest(request); err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.Header.AttestationType, request.Body.Header.SourceId); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
		}
		cleanRequestBodyHex := strings.TrimPrefix(request.Body.RequestBody, "0x")
		requestBodyBytes, err := hex.DecodeString(cleanRequestBodyHex)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to bytes failed: %v", err))
		}
		requestData, err := teecrypto.AbiDecodeRequestData(requestBodyBytes)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
		}
		responseData, err := verifier.Verify(ctx, requestData)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", err))
		}
		responseDataBytes, err := teecrypto.AbiEncodeResponseData(responseData)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", err))
		}
		return types.NewResponse(types.EncodedResponseBody{
			EncodedResponseBody: HexWith0x(responseDataBytes),
		}), nil
	})
}
