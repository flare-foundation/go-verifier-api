package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func registerOp[T any, R any](
	api huma.API,
	id, method, path string,
	tags []string,
	handler func(ctx context.Context, req *T) (*types.Response[R], error),
) {
	huma.Register(api, huma.Operation{
		OperationID: id,
		Method:      method,
		Path:        path,
		Tags:        tags,
	}, handler)
}

func getVerifierAPIPath(sourceName config.SourceName, attestationType connector.AttestationType, endpoint string) string {
	return fmt.Sprintf("/verifier/%s/%s/%s", strings.ToLower(string(sourceName)), attestationType, endpoint)
}

func getVerifierAPITag(attestationType connector.AttestationType) []string {
	return []string{string(attestationType)}
}

func validateSystemAndRequestAttestationNameAndSourceID(config *config.EncodedAndABI, requestAttestationName string, requestSourceID string) error {
	if requestAttestationName != config.AttestationTypePair.AttestationTypeEncoded.Hex() || requestSourceID != config.SourceIDPair.SourceIDEncoded.Hex() {
		var errorMessage = fmt.Errorf(
			"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s)",
			requestAttestationName, requestSourceID,
			string(config.AttestationTypePair.AttestationType), config.AttestationTypePair.AttestationTypeEncoded,
			config.SourceIDPair.SourceID, config.SourceIDPair.SourceIDEncoded,
		)
		return fmt.Errorf("%w", errorMessage)
	}
	return nil
}

func decodeRequest[T any](requestBody []byte, config *config.EncodedAndABI) (T, error) {
	var zero T
	data, err := abiDecodeRequestData[T](requestBody, config.ABIPair.Request)
	if err != nil {
		return zero, fmt.Errorf("%w", err)
	}
	return data, nil
}

func encodeRequest[T any](data T, config *config.EncodedAndABI) ([]byte, error) {
	return encodeWithABI(data, config.ABIPair.Request, "request")
}

func encodeResponse[T any](data T, config *config.EncodedAndABI) ([]byte, error) {
	return encodeWithABI(data, config.ABIPair.Response, "response")
}

func encodeWithABI[T any](data T, arg abi.Argument, kind string) ([]byte, error) {
	encoded, err := abiEncodeData(data, arg)
	if err != nil {
		return nil, fmt.Errorf("encoding %s data failed: %w", kind, err)
	}
	return encoded, nil
}

func prepareRequestBody[T types.RequestConvertible[I], I any](
	body types.AttestationRequestData[T],
	config *config.EncodedAndABI,
) (hexutil.Bytes, error) {
	requestData, err := body.RequestData.ToInternal()
	if err != nil {
		return nil, fmt.Errorf("converting request body to data failed: %w", err)
	}
	encodedRequest, err := encodeRequest(requestData, config)
	if err != nil {
		return nil, err
	}
	return encodedRequest, nil
}

func abiEncodeData[T any](data T, arg abi.Argument) (hexutil.Bytes, error) {
	encoded, err := structs.Encode(arg, data)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func abiDecodeRequestData[T any](data hexutil.Bytes, arg abi.Argument) (T, error) {
	decode, err := structs.Decode[T](arg, data)
	if err != nil {
		var zero T
		return zero, err
	}
	return decode, nil
}

func warnHuma400(message string, err error) error {
	logger.Warnf("%s: %v", message, err)
	return huma.Error400BadRequest(message + ": " + err.Error())
}

func warnHuma500(message string, err error) error {
	logger.Warnf("%s: %v", message, err)
	return huma.Error500InternalServerError(message + ": " + err.Error())
}
