package testhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"

	"github.com/stretchr/testify/require"
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

func TeeAvailabilityCheckRequestBody(teeID, teeProxyID common.Address, url string, challenge common.Hash) attestationtypes.TeeAvailabilityCheckRequestBody {
	return attestationtypes.TeeAvailabilityCheckRequestBody{
		TeeID:      teeID,
		TeeProxyID: teeProxyID,
		URL:        url,
		Challenge:  challenge,
	}
}

func PMWMultisigAccountConfiguredRequestBody(accountAddress string, publicKeys []hexutil.Bytes, threshold uint64) attestationtypes.PMWMultisigAccountConfiguredRequestBody {
	return attestationtypes.PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: accountAddress,
		PublicKeys:     publicKeys,
		Threshold:      threshold,
	}
}

func PMWPaymentStatusRequestBody(opType common.Hash, senderAddress string, nonce uint64, subNonce uint64) attestationtypes.PMWPaymentStatusRequestBody {
	return attestationtypes.PMWPaymentStatusRequestBody{
		OpType:        opType,
		SenderAddress: senderAddress,
		Nonce:         nonce,
		SubNonce:      subNonce,
	}
}

func EncodedIPMWMultisigAccountConfiguredRequestBody(t *testing.T, accountAddress string, publicKeys [][]byte, threshold uint64) []byte {
	t.Helper()
	reqBody := connector.IPMWMultisigAccountConfiguredRequestBody{
		AccountAddress: accountAddress,
		PublicKeys:     publicKeys,
		Threshold:      threshold,
	}
	result, err := structs.Encode(connector.AttestationTypeArguments[connector.PMWMultisigAccountConfigured].Request, reqBody)
	require.NoError(t, err)
	return result
}

func EncodedIPMWPaymentStatusRequestBody(t *testing.T, opType common.Hash, senderAddress string, nonce uint64, subNonce uint64) []byte {
	t.Helper()
	reqBody := connector.IPMWPaymentStatusRequestBody{
		OpType:        opType,
		SenderAddress: senderAddress,
		Nonce:         nonce,
		SubNonce:      subNonce,
	}
	result, err := structs.Encode(connector.AttestationTypeArguments[connector.PMWPaymentStatus].Request, reqBody)
	require.NoError(t, err)
	return result
}

func EncodedITeeAvailabilityCheckRequestBody(t *testing.T, teeID, teeProxyID common.Address, url string, challenge common.Hash) []byte {
	t.Helper()
	reqBody := connector.ITeeAvailabilityCheckRequestBody{
		TeeId:      teeID,
		TeeProxyId: teeProxyID,
		Url:        url,
		Challenge:  challenge,
	}
	result, err := structs.Encode(connector.AttestationTypeArguments[connector.AvailabilityCheck].Request, reqBody)
	require.NoError(t, err)
	return result
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

func LoadEncodedAndABI(t *testing.T, attestationType connector.AttestationType, sourceID config.SourceName) *config.EncodedAndABI {
	t.Helper()
	encodedAndABI, err := config.LoadEncodedAndABI(config.EnvConfig{
		APIKeys:         nil,
		AttestationType: attestationType,
		SourceID:        sourceID,
	})
	require.NoError(t, err)
	return &encodedAndABI
}
