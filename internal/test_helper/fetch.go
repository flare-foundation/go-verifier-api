package testhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/stretchr/testify/require"
)

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("failed to close response body: %v", err)
		}
	}()

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

type ChallengeWrapper struct {
	TeeInfo struct {
		Challenge []byte `json:"Challenge"`
	} `json:"teeInfo"`
}

func GetInfoResponse(t *testing.T) ChallengeWrapper {
	var parsed ChallengeWrapper
	err := json.Unmarshal([]byte(teeInfoResponseJSON), &parsed)
	require.NoError(t, err)
	return parsed
}

var teeInfoResponseJSON = `{"teeInfo":{"Challenge":[193,203,43,98,81,253,64,222,232,126,85,104,40,224,86,54,200,196,1,155,200,172,203,109,186,203,232,17,74,53,149,199],"PublicKey":{"X":[86,187,76,226,197,38,246,49,129,204,38,239,175,54,54,243,25,66,214,76,48,214,157,220,112,112,188,36,82,96,77,68],"Y":[7,101,162,33,198,35,215,229,19,206,154,145,88,172,156,218,123,55,210,240,39,195,133,234,114,184,190,185,82,185,181,78]},"InitialSigningPolicyId":1,"InitialSigningPolicyHash":[242,159,60,217,119,145,124,63,183,255,82,115,200,241,182,54,191,118,221,108,32,145,127,61,203,221,170,212,94,24,231,244],"LastSigningPolicyId":1,"LastSigningPolicyHash":[242,159,60,217,119,145,124,63,183,255,82,115,200,241,182,54,191,118,221,108,32,145,127,61,203,221,170,212,94,24,231,244],"State":{"SystemState":"","SystemStateVersion":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"State":"","StateVersion":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]},"TeeTimestamp":1754902115},"attestation":""}`
