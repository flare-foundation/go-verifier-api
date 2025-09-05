package handler

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func RegisterOp[T any, R any](
	api huma.API,
	id, method, path string,
	tags []string,
	skipValidateBody bool, // TODO Check whether we can avoid this (here because huma changes bytes[32] to string)
	handler func(ctx context.Context, req *T) (*types.Response[R], error),
) {
	huma.Register(api, huma.Operation{
		OperationID:      id,
		Method:           method,
		Path:             path,
		Tags:             tags,
		SkipValidateBody: skipValidateBody,
	}, handler)
}

func toIFTdcHubFtdcAttestationRequest(data types.FTDCRequestEncoded) (connector.IFtdcHubFtdcAttestationRequest, error) {
	return connector.IFtdcHubFtdcAttestationRequest{
		Header: connector.IFtdcHubFtdcRequestHeader{
			AttestationType: data.FTDCHeader.AttestationType,
			SourceId:        data.FTDCHeader.SourceId,
			ThresholdBIPS:   data.FTDCHeader.ThresholdBIPS,
		},
		RequestBody: data.RequestBody,
	}, nil
}

func getVerifierAPIPath(sourceName config.SourceName, attestationType connector.AttestationType, endpoint string) string {
	return fmt.Sprintf("/verifier/%s/%s/%s", strings.ToLower(string(sourceName)), attestationType, endpoint)
}

func getVerifierAPITag(attestationType connector.AttestationType) []string {
	return []string{string(attestationType)}
}

func validateAndParseFTDCRequest[T any](request connector.IFtdcHubFtdcAttestationRequest, config *config.EncodedAndAbi) (T, error) {
	var empty T
	if err := validation.ValidateRequest(request); err != nil {
		return empty, huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(
		config.AttestationTypePair,
		config.SourceIdPair,
		utils.BytesToHex0x(request.Header.AttestationType[:]),
		utils.BytesToHex0x(request.Header.SourceId[:]),
	); err != nil {
		return empty, huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
	}
	data, err := utils.AbiDecodeRequestData[T](request.RequestBody, config.AbiPair.Request)
	if err != nil {
		return empty, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	return data, nil
}

func handleVerifierResult[T any](verifierErr error, responseData T, config *config.EncodedAndAbi) (T, hexutil.Bytes, error) {
	var empty T
	if verifierErr != nil {
		return empty, nil, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", verifierErr))
	}
	responseBytes, verifierErr := utils.AbiEncodeData[T](responseData, config.AbiPair.Response)
	if verifierErr != nil {
		return empty, nil, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", verifierErr))
	}
	return responseData, responseBytes, nil
}

func validatePrepareResponseBody[T any](request types.FTDCRequest[T], config *config.EncodedAndAbi) error {
	if err := validation.ValidateRequest(request); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(
		config.AttestationTypePair,
		config.SourceIdPair,
		request.FTDCHeader.AttestationType.Hex(),
		request.FTDCHeader.SourceId.Hex(),
	); err != nil {
		return huma.Error500InternalServerError(fmt.Sprintf("Request validation failed: %v", err))
	}
	return nil
}

func prepareRequestBody[T any](requestData T, config *config.EncodedAndAbi) (*types.Response[types.EncodedRequestBody], error) {
	// TODO-later add validation (later, now just use it as a helper to generate abi encoded request)
	requestDataBytes, err := utils.AbiEncodeData[T](requestData, config.AbiPair.Request)
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Encoding request data failed: %v", err))
	}
	return types.NewResponse(types.EncodedRequestBody{
		RequestBody: requestDataBytes,
	}), nil
}

func prepareResponseBody[T any, R any, E any](
	ctx context.Context,
	request types.FTDCRequestEncoded,
	validateAndVerify func(connector.IFtdcHubFtdcAttestationRequest, context.Context, *config.EncodedAndAbi, verifierinterface.VerifierInterface[T, R]) (R, []byte, error),
	toExternal func(R) E,
	config *config.EncodedAndAbi,
	verifier verifierinterface.VerifierInterface[T, R]) (*types.Response[types.RawAndEncodedFTDCResponse[E]], error) {
	attestationRequest, err := toIFTdcHubFtdcAttestationRequest(request)
	if err != nil {
		return nil, err
	}
	responseData, responseBytes, err := validateAndVerify(attestationRequest, ctx, config, verifier)
	if err != nil {
		return nil, err
	}
	return &types.Response[types.RawAndEncodedFTDCResponse[E]]{
		Body: types.RawAndEncodedFTDCResponse[E]{
			ResponseData: toExternal(responseData),
			ResponseBody: responseBytes,
		},
	}, nil
}

func logPMWMultisigAccountResponse(response connector.IPMWMultisigAccountConfiguredResponseBody) {
	logger.Debugf("PMWMultisigAccount result: Status=%d, Sequence=%d",
		response.Status, response.Sequence)
}

func logPMWPaymentStatusResponse(response connector.IPMWPaymentStatusResponseBody) {
	logger.Debugf("PMWPaymentStatus result: Sender=%s, Recipient=%s, Amount=%v, Fee=%v, Reference=%x, Status=%d, Revert=%s, Received=%v, TxFee=%v, TxID=%x, Block=%d, Timestamp=%d",
		response.SenderAddress,
		response.RecipientAddress,
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
	logger.Debugf("TEEAvailability result: Status=%d, Timestamp=%d, CodeHash=%x, Platform=%s, InitialSigningPolicyId:%d, LastSigningPolicyId=%d, State=%v",
		response.Status,
		response.TeeTimestamp,
		response.CodeHash,
		bytes.Trim(response.Platform[:], nullByte),
		response.InitialSigningPolicyId,
		response.LastSigningPolicyId,
		response.State)
}
