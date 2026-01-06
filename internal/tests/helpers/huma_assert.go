package helpers

import (
	"io"
	"net/http"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/stretchr/testify/require"
)

// AssertHumaError checks that the response has the expected HTTP status and error message substring.
func AssertHumaError(t *testing.T, resp *http.Response, expectedStatus int, expectedMsg string) {
	t.Helper()
	require.Equal(t, expectedStatus, resp.StatusCode, "unexpected HTTP status")

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body: %v", err)
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)

	require.NoError(t, err, "failed to read response body")
	require.Contains(t, string(bodyBytes), expectedMsg, "error message not found in raw body")
}
