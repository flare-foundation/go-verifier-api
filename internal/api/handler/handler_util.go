package handler

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

func RegisterOp[T any, R any](
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

func ValidateSystemAndRequestAttestationNameAndSourceID(config *config.EncodedAndABI, requestAttestationName string, requestSourceID string) error {
	if requestAttestationName != config.AttestationTypePair.AttestationTypeEncoded.Hex() || requestSourceID != config.SourceIDPair.SourceIDEncoded.Hex() {
		var errorMessage = fmt.Errorf(
			"attestation type and source id combination not supported: (%s, %s). This source supports attestation type '%s' (%s) and source id '%s' (%s)",
			requestAttestationName, requestSourceID,
			string(config.AttestationTypePair.AttestationType), config.AttestationTypePair.AttestationTypeEncoded,
			config.SourceIDPair.SourceID, config.SourceIDPair.SourceIDEncoded,
		)
		return huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", errorMessage))
	}
	return nil
}

func DecodeRequest[T any](requestBody []byte, config *config.EncodedAndABI) (T, error) {
	var zero T
	data, err := abiDecodeRequestData[T](requestBody, config.ABIPair.Request)
	if err != nil {
		return zero, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	return data, nil
}

func EncodeRequest[T any](data T, config *config.EncodedAndABI) ([]byte, error) {
	return encodeWithABI(data, config.ABIPair.Request, "request")
}

func EncodeResponse[T any](data T, config *config.EncodedAndABI) ([]byte, error) {
	return encodeWithABI(data, config.ABIPair.Response, "response")
}

func encodeWithABI[T any](data T, arg abi.Argument, kind string) ([]byte, error) {
	encoded, err := abiEncodeData(data, arg)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Encoding %s data failed: %v", kind, err))
	}
	return encoded, nil
}

func PrepareRequestBody[T types.InternalConvertible[I], I any](
	body types.AttestationRequestData[T],
	config *config.EncodedAndABI,
) (hexutil.Bytes, error) {
	requestData, err := body.RequestData.ToInternal()
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Converting request body to data failed: %v", err))
	}
	encodedRequest, err := EncodeRequest(requestData, config)
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Encoding request data failed: %v", err))
	}
	return encodedRequest, nil
}

func logPMWMultisigAccountResponse(response connector.IPMWMultisigAccountConfiguredResponseBody) {
	logger.Debugf("PMWMultisigAccountConfigured result: Status=%d, Sequence=%d",
		response.Status, response.Sequence)
}

func logPMWPaymentStatusResponse(response connector.IPMWPaymentStatusResponseBody) {
	logger.Debugf("PMWPaymentStatus result: Recipient=%s, TokenID=%v, Amount=%v, Fee=%v, Reference=%x, Status=%d, Revert=%s, Received=%v, TxFee=%v, TxID=%x, Block=%d, Timestamp=%d",
		response.RecipientAddress,
		response.TokenId,
		response.Amount,
		response.Fee,
		response.PaymentReference,
		response.TransactionStatus,
		response.RevertReason,
		response.ReceivedAmount,
		response.TransactionFee,
		response.TransactionId,
		response.BlockNumber,
		response.BlockTimestamp,
	)
}

func logTeeAvailabilityCheckResponse(response connector.ITeeAvailabilityCheckResponseBody) {
	const nullByte = "\x00"
	logger.Debugf("TEEAvailabilityCheck result: Status=%d, Timestamp=%d, CodeHash=%x, Platform=%s, InitialSigningPolicyID:%d, LastSigningPolicyID=%d, State=%v",
		response.Status,
		response.TeeTimestamp,
		response.CodeHash,
		bytes.Trim(response.Platform[:], nullByte),
		response.InitialSigningPolicyId,
		response.LastSigningPolicyId,
		response.State)
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
