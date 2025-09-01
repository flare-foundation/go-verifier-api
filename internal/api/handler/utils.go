package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

func toIFTdcHubFtdcAttestationRequest(data types.FTDCRequestEncoded) (connector.IFtdcHubFtdcAttestationRequest, error) {
	encoded, err := hex.DecodeString(utils.RemoveHexPrefix(data.RequestBody))
	if err != nil {
		return connector.IFtdcHubFtdcAttestationRequest{}, err
	}
	return connector.IFtdcHubFtdcAttestationRequest{
		Header: connector.IFtdcHubFtdcRequestHeader{
			AttestationType: common.HexToHash(data.FTDCHeader.AttestationType),
			SourceId:        common.HexToHash(data.FTDCHeader.SourceId),
			ThresholdBIPS:   data.FTDCHeader.ThresholdBIPS,
		},
		RequestBody: encoded,
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
	requestData, err := utils.AbiDecodeRequestData[T](request.RequestBody, config.AbiPair.Request)
	if err != nil {
		return empty, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	return requestData, nil
}

func handleVerifierResult[T any](verifierErr error, responseData T, config *config.EncodedAndAbi) (T, []byte, error) {
	var empty T
	if verifierErr != nil {
		return empty, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Verification failed: %v", verifierErr))
	}
	responseDataBytes, verifierErr := utils.AbiEncodeData[T](responseData, config.AbiPair.Response)
	if verifierErr != nil {
		return empty, []byte{}, huma.Error500InternalServerError(fmt.Sprintf("Encoding response data failed: %v", verifierErr))
	}
	return responseData, responseDataBytes, nil
}

func validatePrepareResponseBody[T any](request types.FTDCRequest[T], config *config.EncodedAndAbi) error {
	if err := validation.ValidateRequest(request); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceId(
		config.AttestationTypePair,
		config.SourceIdPair,
		request.FTDCHeader.AttestationType,
		request.FTDCHeader.SourceId,
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
		RequestBody: utils.BytesToHex0x(requestDataBytes),
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
	responseData, responseDataBytes, err := validateAndVerify(attestationRequest, ctx, config, verifier)
	if err != nil {
		return nil, err
	}
	return &types.Response[types.RawAndEncodedFTDCResponse[E]]{
		Body: types.RawAndEncodedFTDCResponse[E]{
			ResponseData: toExternal(responseData),
			ResponseBody: utils.BytesToHex0x(responseDataBytes),
		},
	}, nil
}

func logPMWMultisigAccountResponse(response connector.IPMWMultisigAccountConfiguredResponseBody) {
	logger.Debugf("Result after PMWMultisigAccount verification: Status=%d, Sequence=%d",
		response.Status, response.Sequence)
}

func logPMWPaymentStatusResponse(response connector.IPMWPaymentStatusResponseBody) {
	logger.Debugf("Result after PMWPaymentStatusRequest verification: SenderAddress=%s, RecipientAddress=%s, Amount=%v, Fee=%v, PaymentReference=%x, TransactionStatus=%d, RevertReason=%s, ReceivedAmount=%v, TransactionFee=%v, TransactionId=%x, BlockNumber=%d, BlockTimestamp=%d",
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
	logger.Debugf("Result of TEEAvailability verification: Status=%d, Timestamp=%d, CodeHash=%x, Platform=%s, InitialSigningPolicyId:%d, LastSigningPolicyId=%d, State=%v",
		response.Status,
		response.TeeTimestamp,
		response.CodeHash,
		bytes.Trim(response.Platform[:], "\x00"),
		response.InitialSigningPolicyId,
		response.LastSigningPolicyId,
		response.State)
}
