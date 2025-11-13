package testhelper

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
)

func CreateAttestationRequest(t *testing.T, attestationType, sourceID common.Hash, reqBody []byte) attestationtypes.AttestationRequest {
	t.Helper()
	return attestationtypes.AttestationRequest{
		AttestationType: attestationType,
		SourceID:        sourceID,
		RequestBody:     reqBody,
	}
}

func CreateAttestationRequestData[T any](t *testing.T, attestationType common.Hash, sourceID common.Hash, requestData T) attestationtypes.AttestationRequestData[T] {
	t.Helper()
	return attestationtypes.AttestationRequestData[T]{
		AttestationType: attestationType,
		SourceID:        sourceID,
		RequestData:     requestData,
	}
}

func TeeAvailabilityCheckRequestBody(t *testing.T, data connector.ITeeAvailabilityCheckRequestBody) attestationtypes.TeeAvailabilityCheckRequestBody {
	t.Helper()
	return attestationtypes.TeeAvailabilityCheckRequestBody{
		TeeID:         data.TeeId,
		TeeProxyID:    data.TeeProxyId,
		URL:           data.Url,
		Challenge:     data.Challenge,
		InstructionID: data.InstructionId,
	}
}

func PMWMultisigAccountConfiguredRequestBody(t *testing.T, data connector.IPMWMultisigAccountConfiguredRequestBody) attestationtypes.PMWMultisigAccountConfiguredRequestBody {
	t.Helper()
	return attestationtypes.PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: data.AccountAddress,
		PublicKeys:     toHexutilBytesSlice(t, data.PublicKeys),
		Threshold:      data.Threshold,
	}
}

func PMWPaymentStatusRequestBody(t *testing.T, data connector.IPMWPaymentStatusRequestBody) attestationtypes.PMWPaymentStatusRequestBody {
	t.Helper()
	return attestationtypes.PMWPaymentStatusRequestBody{
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
