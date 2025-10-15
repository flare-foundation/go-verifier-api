package testhelper

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// AssertHumaError checks that the response has the expected HTTP status and error message substring.
func AssertHumaError(t *testing.T, resp *http.Response, expectedStatus int, expectedMsg string) {
	t.Helper()

	require.Equal(t, expectedStatus, resp.StatusCode, "unexpected HTTP status")

	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)

	require.NoError(t, err, "failed to read response body")
	require.Contains(t, string(bodyBytes), expectedMsg, "error message not found in raw body")
}
