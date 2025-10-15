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
	t.Helper()
	var empty T
	resp, err := doRequest(t, http.MethodPost, url, apiKey, data)
	if err != nil {
		return empty, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body: %v", err)
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
	t.Helper()
	return doRequest(t, http.MethodPost, url, apiKey, data)
}

func Get(t *testing.T, url, apiKey string) ([]byte, error) {
	t.Helper()
	resp, err := doRequest(t, http.MethodGet, url, apiKey, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
