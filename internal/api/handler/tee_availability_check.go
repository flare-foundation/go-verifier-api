package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "gitlab.com/urskak/verifier-api/internal/api/type"
	teecrypto "gitlab.com/urskak/verifier-api/internal/attestation/tee_availability_check/crypto"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

func TeeAvailabilityCheckHandler(api huma.API, attestationType connector.AttestationType, verifier verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, types.TeeAvailabilityResponseData], sourceID string) {
	huma.Post(api, fmt.Sprintf("/%s/%s", attestationType, "prepareRequestBody"), func(ctx context.Context, request *struct {
		Body types.TeeAvailabilityRequest
	}) (*types.Response[types.EncodedRequestBody], error) {
		if err := ValidateRequest(request); err != nil {
			return nil, err
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		internalBody, err := request.Body.RequestBody.ToInternal()
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("conversion failed: %v", err))
		}
		res, err := teecrypto.AbiEncodeRequestBody(internalBody)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("encoding failed: %v", err))
		}
		return types.NewResponse(types.EncodedRequestBody{
			EncodedRequestBody: HexWith0x(res),
		}), nil
	})

	huma.Post(api, fmt.Sprintf("/%s/%s", attestationType, "prepareResponseBody"), func(ctx context.Context, request *struct {
		Body types.TeeAvailabilityRequest
	}) (*types.Response[types.EncodedResponseBody], error) {
		if err := ValidateRequest(request); err != nil {
			return nil, err
		}
		if err := ValidateSystemAndRequestAttestationNameAndSourceId(attestationType, sourceID, request.Body.AttestationType, request.Body.SourceId); err != nil {
			return nil, err
		}
		// TODO verify
		// TODO prepare encoded and decoded response body
		return nil, huma.Error501NotImplemented("TeeAvailabilityChecky - prepareResponseBody")
	})

	huma.Post(api, fmt.Sprintf("/%s/%s", attestationType, "verify"), func(ctx context.Context, request *struct {
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
			return nil, huma.Error400BadRequest(fmt.Sprintf("decoding if request body failed: %v", err))
		}
		_, err = verifier.Verify(ctx, requestData)
		if err != nil {
			return nil, huma.NewError(http.StatusBadRequest, fmt.Sprintf("verification failed: %v", err))
		}
		// responseDataBytes, err := teecrypto.AbiEncodeRequestBody(responseData)
		//TODO - encode actual response
		response := types.EncodedResponseBody{EncodedResponseBody: HexWith0x([]byte{})}
		return types.NewResponse(response), nil
	})
}
