package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
)

type TestCase[T any, R any] struct {
	Name           string
	Input          T
	ExpectedValue  R
	ExpectError    bool
	ExpectedErrMsg string
}

func CreateIFtdcHubFtdcAttestationRequest(t *testing.T, attestationType, sourceId common.Hash, reqBody []byte) connector.IFtdcHubFtdcAttestationRequest {
	t.Helper()
	return connector.IFtdcHubFtdcAttestationRequest{
		Header: connector.IFtdcHubFtdcRequestHeader{
			AttestationType: attestationType,
			SourceId:        sourceId,
			ThresholdBIPS:   1000,
		},
		RequestBody: reqBody,
	}
}

func EncodedIPMWMultisigAccountConfiguredRequestBody(t *testing.T, walletAddress string, publicKeys [][]byte, threshold uint64) []byte {
	t.Helper()
	reqBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: walletAddress,
		PublicKeys:    publicKeys,
		Threshold:     threshold,
	}
	result, err := structs.Encode(connector.AttestationTypeArguments[connector.PMWMultisigAccountConfigured].Request, reqBody)
	require.NoError(t, err)
	return result
}

func EncodedIPMWPaymentStatusRequestBody(t *testing.T, walletId common.Hash, nonce uint64, subNonce uint64) []byte {
	t.Helper()
	reqBody := connector.IPMWPaymentStatusRequestBody{
		WalletId: walletId,
		Nonce:    nonce,
		SubNonce: subNonce,
	}
	result, err := structs.Encode(connector.AttestationTypeArguments[connector.PMWPaymentStatus].Request, reqBody)
	require.NoError(t, err)
	return result
}

func EncodedITeeAvailabilityCheckRequestBody(t *testing.T, teeId common.Address, url string, challenge common.Hash) []byte {
	t.Helper()
	reqBody := connector.ITeeAvailabilityCheckRequestBody{
		TeeId:     teeId,
		Url:       url,
		Challenge: challenge,
	}
	result, err := structs.Encode(connector.AttestationTypeArguments[connector.AvailabilityCheck].Request, reqBody)
	require.NoError(t, err)
	return result
}

func FTDCRequestEncoded(t *testing.T, attestationType common.Hash, sourceId common.Hash, requestBody []byte) attestationtypes.FTDCRequestEncoded {
	t.Helper()
	return attestationtypes.FTDCRequestEncoded{
		FTDCHeader: attestationtypes.FTDCHeader{
			AttestationType: attestationType,
			SourceId:        sourceId,
			ThresholdBIPS:   0,
		},
		RequestBody: requestBody,
	}
}

func InternalFTDCRequest[T any](t *testing.T, attestationType common.Hash, sourceId common.Hash, requestData T) attestationtypes.FTDCRequest[T] {
	t.Helper()
	return attestationtypes.FTDCRequest[T]{
		FTDCHeader: attestationtypes.FTDCHeader{
			AttestationType: attestationType,
			SourceId:        sourceId,
			ThresholdBIPS:   0,
		},
		RequestData: requestData,
	}
}

func EncodeFTDCPMWMultisigAccountConfiguredRequest(t *testing.T, data connector.IPMWMultisigAccountConfiguredRequestBody) []byte {
	t.Helper()
	result, err := structs.Encode(connector.AttestationTypeArguments[connector.PMWMultisigAccountConfigured].Request, data)
	require.NoError(t, err)
	return result
}

func DecodeFTDCTeeAvailabilityCheckResponse(t *testing.T, data []byte) connector.ITeeAvailabilityCheckResponseBody {
	t.Helper()
	var request connector.ITeeAvailabilityCheckResponseBody
	err := structs.DecodeTo(connector.AttestationTypeArguments[connector.AvailabilityCheck].Response, data, &request)
	require.NoError(t, err)
	return request
}

func DecodeFTDCPMVPaymentStatusResponse(t *testing.T, data []byte) connector.IPMWPaymentStatusResponseBody {
	t.Helper()
	var request connector.IPMWPaymentStatusResponseBody
	err := structs.DecodeTo(connector.AttestationTypeArguments[connector.PMWPaymentStatus].Response, data, &request)
	require.NoError(t, err)
	return request
}

func DecodeFTDCPMWMultisigAccountConfiguredResponse(t *testing.T, data []byte) connector.IPMWMultisigAccountConfiguredResponseBody {
	t.Helper()
	var request connector.IPMWMultisigAccountConfiguredResponseBody
	err := structs.DecodeTo(connector.AttestationTypeArguments[connector.PMWMultisigAccountConfigured].Response, data, &request)
	require.NoError(t, err)
	return request
}

func Post[T any](t *testing.T, url string, data interface{}, apiKey string) (T, error) {
	t.Helper()
	var empty T
	payload, err := json.Marshal(data)
	if err != nil {
		return empty, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return empty, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("error response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, err
	}

	var response T
	if err := json.Unmarshal(body, &response); err != nil {
		return empty, err
	}

	return response, nil
}

func PostWithoutMarshalling(t *testing.T, url string, data interface{}, apiKey string) (*http.Response, error) {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return resp, nil
}

func Get(t *testing.T, url string, apiKey string) ([]byte, error) {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
