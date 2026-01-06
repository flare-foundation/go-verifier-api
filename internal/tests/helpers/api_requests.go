package helpers

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
)

func CreateAttestationRequest(t *testing.T, attestationType, sourceID common.Hash, reqBody []byte) types.AttestationRequest {
	t.Helper()
	return types.AttestationRequest{
		AttestationType: attestationType,
		SourceID:        sourceID,
		RequestBody:     reqBody,
	}
}

func CreateAttestationRequestData[T any](t *testing.T, attestationType common.Hash, sourceID common.Hash, requestData T) types.AttestationRequestData[T] {
	t.Helper()
	return types.AttestationRequestData[T]{
		AttestationType: attestationType,
		SourceID:        sourceID,
		RequestData:     requestData,
	}
}

func TeeAvailabilityCheckRequestBody(t *testing.T, data connector.ITeeAvailabilityCheckRequestBody) types.TeeAvailabilityCheckRequestBody {
	t.Helper()
	return types.TeeAvailabilityCheckRequestBody{
		TeeID:         data.TeeId,
		TeeProxyID:    data.TeeProxyId,
		URL:           data.Url,
		Challenge:     data.Challenge,
		InstructionID: data.InstructionId,
	}
}

func PMWMultisigAccountConfiguredRequestBody(t *testing.T, data connector.IPMWMultisigAccountConfiguredRequestBody) types.PMWMultisigAccountConfiguredRequestBody {
	t.Helper()
	return types.PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: data.AccountAddress,
		PublicKeys:     toHexutilBytesSlice(t, data.PublicKeys),
		Threshold:      data.Threshold,
	}
}

func PMWPaymentStatusRequestBody(t *testing.T, data connector.IPMWPaymentStatusRequestBody) types.PMWPaymentStatusRequestBody {
	t.Helper()
	return types.PMWPaymentStatusRequestBody{
		OpType:        data.OpType,
		SenderAddress: data.SenderAddress,
		Nonce:         data.Nonce,
		SubNonce:      data.SubNonce,
	}
}

func toHexutilBytesSlice(t *testing.T, src [][]byte) []hexutil.Bytes {
	t.Helper()
	res := make([]hexutil.Bytes, len(src))
	for i, b := range src {
		res[i] = hexutil.Bytes(b)
	}
	return res
}
