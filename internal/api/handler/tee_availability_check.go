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
			return nil, err
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		requestData, err := request.Body.RequestBody.ToInternal()
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("converting request body failed: %v", err))
		}
		requestDataBytes, err := teecrypto.AbiEncodeRequestData(requestData)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("encoding request body failed: %v", err))
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
			return nil, err
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		cleanRequestBodyHex := strings.TrimPrefix(request.Body.RequestBody, "0x")
		requestBodyBytes, err := hex.DecodeString(cleanRequestBodyHex)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("decoding request body to bytes failed: %v", err))
		}
		requestData, err := teecrypto.AbiDecodeRequestData(requestBodyBytes)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("decoding request body failed: %v", err))
		}
		responseData, err := verifier.Verify(ctx, requestData)
		if err != nil {
			return nil, err
		}
		responseBody := responseData.ToExternal()
		responseDataBytes, err := teecrypto.AbiEncodeResponseData(responseData)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("encoding response body failed: %v", err))
		}
		return types.NewResponse(types.RawAndEncodedResponseBody{
			ResponseBody:        responseBody,
			EncodedResponseBody: HexWith0x(responseDataBytes),
		}), nil
	})
	// verify
	huma.Post(api, fmt.Sprintf("/%s/verify", attestationType), func(ctx context.Context, request *struct {
		Body types.TeeAvailabilityEncodedRequest
	}) (*types.Response[types.EncodedResponseBody], error) {
		if err := ValidateRequest(request); err != nil {
			return nil, err
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		cleanRequestBodyHex := strings.TrimPrefix(request.Body.RequestBody, "0x")
		requestBodyBytes, err := hex.DecodeString(cleanRequestBodyHex)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("decoding request body to bytes failed: %v", err))
		}
		requestData, err := teecrypto.AbiDecodeRequestData(requestBodyBytes)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("decoding request body failed: %v", err))
		}
		responseData, err := verifier.Verify(ctx, requestData)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("verification failed: %v", err))
		}
		responseDataBytes, err := teecrypto.AbiEncodeResponseData(responseData)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("encoding response body failed: %v", err))
		}
		return types.NewResponse(types.EncodedResponseBody{
			EncodedResponseBody: HexWith0x(responseDataBytes),
		}), nil
	})
}
