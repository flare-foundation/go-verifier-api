package helpers

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/stretchr/testify/require"
)

// AssertVerifierStatus checks that a /verify response is HTTP 200 carrying the
// status-based envelope with the expected status, and returns the decoded
// envelope so callers can make further assertions (e.g. on ResponseBody).
func AssertVerifierStatus(t *testing.T, resp *http.Response, expectedStatus string) types.VerifierResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.StatusCode, "unexpected HTTP status")
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body: %v", err)
		}
	}()

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")

	var env types.VerifierResponse
	require.NoError(t, json.Unmarshal(b, &env), "failed to decode verifier envelope")
	require.Equal(t, expectedStatus, env.Status, "unexpected verifier status")

	return env
}
