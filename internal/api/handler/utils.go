package handler

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
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
