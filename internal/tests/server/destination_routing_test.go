package server_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	server "github.com/flare-foundation/go-verifier-api/internal/tests/server"
	"github.com/stretchr/testify/require"
)

// postStatus posts an empty JSON body to url with the API key and returns the
// HTTP status. Routing is what is under test: an existing route answers with a
// validation status (400/422), never 404.
func postStatus(t *testing.T, url, apiKey string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode
}

// TestDestinationRouting pins the four-part route namespace: the configured
// destination's routes exist, while the legacy three-part route and any other
// destination's routes are 404 — two deployments for the same source but
// different destination chains expose distinct URLs.
func TestDestinationRouting(t *testing.T) {
	config.ClearPMWMultisigAccountConfiguredConfigForTest()
	setup := server.SetupServer(t, fdc2.PMWMultisigAccountConfigured, config.SourceTestXRP, config.EnvConfig{
		SourceRPCURL:            "https://s.altnet.rippletest.net:51234",
		DestinationChainURLSlug: "coston",
	})

	base := "http://localhost:" + setup.Port
	configured := base + "/verifier/testxrp/coston/PMWMultisigAccountConfigured/verify"
	legacy := base + "/verifier/testxrp/PMWMultisigAccountConfigured/verify"
	wrongDestination := base + "/verifier/testxrp/songbird/PMWMultisigAccountConfigured/verify"

	require.NotEqual(t, http.StatusNotFound, postStatus(t, configured, setup.APIKey),
		"the configured destination's route must exist")
	require.Equal(t, http.StatusNotFound, postStatus(t, legacy, setup.APIKey),
		"the legacy route without a destination segment must not be registered")
	require.Equal(t, http.StatusNotFound, postStatus(t, wrongDestination, setup.APIKey),
		"a request using the wrong destination must receive 404")
	setup.Stop()

	// A second deployment for the same source but a different destination chain
	// serves its own distinct route namespace with the same backend configuration.
	config.ClearPMWMultisigAccountConfiguredConfigForTest()
	setup2 := server.SetupServer(t, fdc2.PMWMultisigAccountConfigured, config.SourceTestXRP, config.EnvConfig{
		SourceRPCURL:            "https://s.altnet.rippletest.net:51234",
		DestinationChainURLSlug: "songbird",
	})
	defer setup2.Stop()

	require.NotEqual(t, http.StatusNotFound, postStatus(t, wrongDestination, setup2.APIKey),
		"the songbird deployment must serve the songbird route")
	require.Equal(t, http.StatusNotFound, postStatus(t, configured, setup2.APIKey),
		"the songbird deployment must not serve the coston route")
}
