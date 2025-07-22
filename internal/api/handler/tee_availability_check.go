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
	teecrypto "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/crypto"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func TeeAvailabilityCheckHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], sourceID string) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/prepareRequestBody", attestationType),
		Tags:        []string{string(attestationType)}},
		func(ctx context.Context, request *struct {
			Body types.TeeAvailabilityRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := ValidateRequest(request); err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
			}
			if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.FTDCHeader.AttestationType, request.Body.FTDCHeader.SourceId); err != nil {
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
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareResponseBody",
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/prepareResponseBody", attestationType),
		Tags:        []string{string(attestationType)}},
		func(ctx context.Context, request *struct {
			Body types.TeeAvailabilityEncodedRequest
		}) (*types.Response[types.RawAndEncodedResponseBody], error) {
			responseData, responseDataBytes, err := validateAndVerifyEncodedRequest(request.Body, ctx, attestationType, verifier, sourceID)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.RawAndEncodedResponseBody{
				ResponseBody:        responseData.ToExternal(),
				EncodedResponseBody: HexWith0x(responseDataBytes),
			}), nil
		})
	// verify
	huma.Register(api, huma.Operation{
		OperationID: "post-verify",
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/verify", attestationType),
		Tags:        []string{string(attestationType)}},
		func(ctx context.Context, request *struct {
			Body types.TeeAvailabilityEncodedRequest
		}) (*types.Response[types.EncodedResponseBody], error) {
			_, responseDataBytes, err := validateAndVerifyEncodedRequest(request.Body, ctx, attestationType, verifier, sourceID)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.EncodedResponseBody{
				EncodedResponseBody: HexWith0x(responseDataBytes),
			}), nil
		})
}

// func validate

func validateAndVerifyEncodedRequest(request types.TeeAvailabilityEncodedRequest, ctx context.Context, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], sourceID string) (types.TeeAvailabilityResponseData, []byte, error) {
	if err := ValidateRequest(request); err != nil {
		return types.TeeAvailabilityResponseData{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.FTDCHeader.AttestationType, request.FTDCHeader.SourceId); err != nil {
		return types.TeeAvailabilityResponseData{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
	}
	cleanRequestBodyHex := strings.TrimPrefix(request.RequestBody, "0x")
	requestBodyBytes, err := hex.DecodeString(cleanRequestBodyHex)
	if err != nil {
		return types.TeeAvailabilityResponseData{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to bytes failed: %v", err))
	}
	requestData, err := teecrypto.AbiDecodeRequestData(requestBodyBytes)
	if err != nil {
		return types.TeeAvailabilityResponseData{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	responseData, err := verifier.Verify(ctx, requestData)
	if err != nil {
		return types.TeeAvailabilityResponseData{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", err))
	}
	responseDataBytes, err := teecrypto.AbiEncodeResponseData(responseData)
	if err != nil {
		return types.TeeAvailabilityResponseData{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", err))
	}
	return responseData, responseDataBytes, nil
}
