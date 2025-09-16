package testhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"

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

func TeeAvailabilityCheckRequestBody(data connector.ITeeAvailabilityCheckRequestBody) attestationtypes.TeeAvailabilityCheckRequestBody {
	return attestationtypes.TeeAvailabilityCheckRequestBody{
		TeeID:      data.TeeId,
		TeeProxyID: data.TeeProxyId,
		URL:        data.Url,
		Challenge:  data.Challenge,
	}
}

func PMWMultisigAccountConfiguredRequestBody(data connector.IPMWMultisigAccountConfiguredRequestBody) attestationtypes.PMWMultisigAccountConfiguredRequestBody {
	return attestationtypes.PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: data.AccountAddress,
		PublicKeys:     toHexutilBytesSlice(data.PublicKeys),
		Threshold:      data.Threshold,
	}
}

func toHexutilBytesSlice(src [][]byte) []hexutil.Bytes {
	res := make([]hexutil.Bytes, len(src))
	for i, b := range src {
		res[i] = hexutil.Bytes(b)
	}
	return res
}

func PMWPaymentStatusRequestBody(data connector.IPMWPaymentStatusRequestBody) attestationtypes.PMWPaymentStatusRequestBody {
	return attestationtypes.PMWPaymentStatusRequestBody{
		OpType:        data.OpType,
		SenderAddress: data.SenderAddress,
		Nonce:         data.Nonce,
		SubNonce:      data.SubNonce,
	}
}

func EncodeRequestBody[T any](t *testing.T, attType connector.AttestationType, body T) []byte {
	t.Helper()
	result, err := structs.Encode(connector.AttestationTypeArguments[attType].Request, body)
	require.NoError(t, err)
	return result
}

func DecodeResponseBody[T any](t *testing.T, attType connector.AttestationType, data []byte) T {
	t.Helper()
	var resp T
	err := structs.DecodeTo(connector.AttestationTypeArguments[attType].Response, data, &resp)
	require.NoError(t, err)
	return resp
}

func doRequest(t *testing.T, method, url, apiKey string, payload interface{}) (*http.Response, error) {
	t.Helper()

	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		require.NoError(t, err)
		body = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err)

	if apiKey != "" {
		req.Header.Set("X-API-KEY", apiKey)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return http.DefaultClient.Do(req)
}

func Post[T any](t *testing.T, url string, data interface{}, apiKey string) (T, error) {
	var empty T
	resp, err := doRequest(t, http.MethodPost, url, apiKey, data)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("error response status: %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, err
	}

	var result T
	err = json.Unmarshal(b, &result)
	return result, err
}

func PostWithoutMarshalling(t *testing.T, url string, data interface{}, apiKey string) (*http.Response, error) {
	return doRequest(t, http.MethodPost, url, apiKey, data)
}

func Get(t *testing.T, url, apiKey string) ([]byte, error) {
	resp, err := doRequest(t, http.MethodGet, url, apiKey, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
