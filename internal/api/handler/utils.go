package handler

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/api/validation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
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

func ValidateRequestData[T any](request types.AttestationRequestData[T], config *config.EncodedAndABI) error {
	if err := validation.ValidateRequest(request); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceID(
		config.AttestationTypePair,
		config.SourceIDPair,
		request.AttestationType.Hex(),
		request.SourceID.Hex(),
	); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	return nil
}

func ValidateRequest(request types.AttestationRequest, config *config.EncodedAndABI) error {
	if err := validation.ValidateRequest(request); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	if err := validation.ValidateSystemAndRequestAttestationNameAndSourceID(
		config.AttestationTypePair,
		config.SourceIDPair,
		request.AttestationType.Hex(),
		request.SourceID.Hex(),
	); err != nil {
		return huma.Error400BadRequest(fmt.Sprintf("Request validation failed: %v", err))
	}
	return nil
}

func DecodeRequest[T any](requestBody []byte, config *config.EncodedAndABI) (T, error) {
	var zero T
	data, err := utils.ABIDecodeRequestData[T](requestBody, config.ABIPair.Request)
	if err != nil {
		return zero, huma.Error400BadRequest(fmt.Sprintf("Decoding request body to data failed: %v", err))
	}
	return data, nil
}

func EncodeResponse[T any](responseData T, config *config.EncodedAndABI) ([]byte, error) {
	data, err := utils.ABIEncodeData(responseData, config.ABIPair.Response)
	if err != nil {
		return []byte{}, huma.Error400BadRequest(fmt.Sprintf("Encoding response data failed: %v", err))
	}
	return data, nil
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
	logger.Debugf("TEEAvailability result: Status=%d, Timestamp=%d, CodeHash=%x, Platform=%s, InitialSigningPolicyID:%d, LastSigningPolicyID=%d, State=%v",
		response.Status,
		response.TeeTimestamp,
		response.CodeHash,
		bytes.Trim(response.Platform[:], nullByte),
		response.InitialSigningPolicyId,
		response.LastSigningPolicyId,
		response.State)
}
