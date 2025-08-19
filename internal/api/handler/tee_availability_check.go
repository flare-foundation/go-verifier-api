package handler

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	teeverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func TeeAvailabilityCheckHandler(api huma.API, config config.TeeAvailabilityCheckConfig, verifier verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, connector.ITeeAvailabilityCheckResponseBody]) {
	// prepare RequestBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareRequestBody",
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/prepareRequestBody", config.AttestationTypePair.AttestationType),
		Tags:        []string{string(config.AttestationTypePair.AttestationType)}},
		func(ctx context.Context, request *struct {
			Body types.TeeAvailabilityRequest
		}) (*types.Response[types.EncodedRequestBody], error) {
			if err := validation.ValidateRequest(request); err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
			}
			if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(config.AttestationTypePair, config.SourcePair, request.Body.FTDCHeader.AttestationType, request.Body.FTDCHeader.SourceId); err != nil {
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
			}
			requestData, err := request.Body.RequestData.ToInternal()
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
			}
			// TODO-later add validation (later, now just use it as a helper to generate abi encoded request)
			requestDataBytes, err := utils.AbiEncodeRequestData[types.TeeAvailabilityRequestData](requestData, config.AbiPair.Request)
			if err != nil {
				return nil, huma.Error400BadRequest(fmt.Sprintf("Encoding request data failed: %v", err))
			}
			return types.NewResponse(types.EncodedRequestBody{
				RequestBody: utils.HexWith0x(requestDataBytes),
			}), nil
		})
	// prepare ResponseBody
	huma.Register(api, huma.Operation{
		OperationID: "post-prepareResponseBody",
		Method:      http.MethodPost,
		Path:        fmt.Sprintf("/%s/prepareResponseBody", config.AttestationTypePair.AttestationType),
		Tags:        []string{string(config.AttestationTypePair.AttestationType)}},
		func(ctx context.Context, request *struct {
			Body types.TeeAvailabilityEncodedRequest
		}) (*types.Response[types.RawAndEncodedTeeAvailabilityResponseBody], error) {
			attestationRequest, err := toIFTdcHubFtdcAttestationRequest(request.Body)
			if err != nil {
				return nil, err
			}
			responseData, responseDataBytes, err := validateAndVerifyEncodedRequest(attestationRequest, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.RawAndEncodedTeeAvailabilityResponseBody{
				ResponseData: types.ToExternal(responseData),
				ResponseBody: utils.HexWith0x(responseDataBytes),
			}), nil
		})
	// verify
	huma.Register(api, huma.Operation{
		OperationID:      "post-verify",
		Method:           http.MethodPost,
		Path:             fmt.Sprintf("/%s/verify", config.AttestationTypePair.AttestationType),
		Tags:             []string{string(config.AttestationTypePair.AttestationType)},
		SkipValidateBody: true, // TODO Check whether we can avoid this (here because huma changes bytes[32] to string)
	},

		func(ctx context.Context, request *struct {
			Body connector.IFtdcHubFtdcAttestationRequest
		}) (*types.Response[types.EncodedResponseBody], error) {
			_, responseDataBytes, err := validateAndVerifyEncodedRequest(request.Body, ctx, config, verifier)
			if err != nil {
				return nil, err
			}
			return types.NewResponse(types.EncodedResponseBody{
				ResponseBody: utils.HexWith0x(responseDataBytes),
			}), nil
		})
}

func validateAndVerifyEncodedRequest(request connector.IFtdcHubFtdcAttestationRequest, ctx context.Context, config config.TeeAvailabilityCheckConfig, verifier verifierinterface.VerifierInterface[types.TeeAvailabilityRequestData, connector.ITeeAvailabilityCheckResponseBody]) (connector.ITeeAvailabilityCheckResponseBody, []byte, error) {
	if err := validation.ValidateRequest(request); err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(config.AttestationTypePair, config.SourcePair, "0x"+hex.EncodeToString(request.Header.AttestationType[:]), "0x"+hex.EncodeToString(request.Header.SourceId[:])); err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
	}
	requestBodyBytes := request.RequestBody
	requestData, err := utils.AbiDecodeRequestData[types.TeeAvailabilityRequestData](requestBodyBytes, config.AbiPair.Request)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	responseData, err := verifier.Verify(ctx, requestData)
	if err != nil {
		if errors.Is(err, teeverifier.ErrIndeterminate) {
			return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, huma.Error503ServiceUnavailable(fmt.Sprintf("Verification failed: %v", err))
		}
		return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", err))
	}
	responseDataBytes, err := utils.AbiEncodeResponseData[connector.ITeeAvailabilityCheckResponseBody](responseData, config.AbiPair.Response)
	if err != nil {
		return connector.ITeeAvailabilityCheckResponseBody{}, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", err))
	}
	return responseData, responseDataBytes, nil
}

func toIFTdcHubFtdcAttestationRequest(data types.TeeAvailabilityEncodedRequest) (connector.IFtdcHubFtdcAttestationRequest, error) {
	encoded, err := hex.DecodeString(strings.TrimPrefix(data.RequestBody, "0x"))
	if err != nil {
		return connector.IFtdcHubFtdcAttestationRequest{}, err
	}
	return connector.IFtdcHubFtdcAttestationRequest{
		Header: connector.IFtdcHubFtdcRequestHeader{
			AttestationType: common.HexToHash(data.FTDCHeader.AttestationType),
			SourceId:        common.HexToHash(data.FTDCHeader.SourceId),
			ThresholdBIPS:   data.FTDCHeader.ThresholdBIPS,
			Cosigners: func(cs []string) []common.Address {
				addrs := make([]common.Address, len(cs))
				for i := range cs {
					addrs[i] = common.HexToAddress(cs[i])
				}
				return addrs
			}(data.FTDCHeader.Cosigners),
			CosignersThreshold: data.FTDCHeader.CosignersThreshold,
		},
		RequestBody: encoded,
	}, nil
}
