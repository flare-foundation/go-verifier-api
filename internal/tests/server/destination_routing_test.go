package server_test

import (
	"bytes"
	"io"
	"net/http"
	"regexp"
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

// getStatus performs an unauthenticated GET and returns the HTTP status.
func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec // test-local URL
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode
}

// TestDeploymentPrefixedDocsAndHealth pins the non-attestation surface under
// the deployment prefix: docs, OpenAPI, and health are keyless there, the old
// root paths are gone, and attestation endpoints stay key-protected.
func TestDeploymentPrefixedDocsAndHealth(t *testing.T) {
	config.ClearPMWMultisigAccountConfiguredConfigForTest()
	setup := server.SetupServer(t, fdc2.PMWMultisigAccountConfigured, config.SourceTestXRP, config.EnvConfig{
		SourceRPCURL:            "https://s.altnet.rippletest.net:51234",
		DestinationChainURLSlug: "sgb",
	})
	defer setup.Stop()

	base := "http://localhost:" + setup.Port
	prefix := base + "/verifier/testxrp/sgb"

	t.Run("prefixed docs, spec, and health serve without an API key", func(t *testing.T) {
		require.Equal(t, http.StatusOK, getStatus(t, prefix+"/api-doc/"))
		require.Equal(t, http.StatusOK, getStatus(t, prefix+"/api-doc/swagger-ui.css"))
		require.Equal(t, http.StatusOK, getStatus(t, prefix+"/api-doc/init-swagger.js"))
		require.Equal(t, http.StatusOK, getStatus(t, prefix+"/openapi.json"))
		require.Equal(t, http.StatusOK, getStatus(t, prefix+"/api/health"))
	})

	t.Run("OpenAPI refs avoid the root path space and prefixed schemas resolve", func(t *testing.T) {
		resp, err := http.Get(prefix + "/openapi.json") //nolint:gosec // test-local URL
		require.NoError(t, err)
		spec, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		// Refs in the document are internal (#/components/...); none may point at
		// the retired root-level schemas path.
		require.NotContains(t, string(spec), `"$ref":"/schemas/`,
			"schema refs must not point at the root path space")
		// Standalone schema documents live under the prefixed SchemasPath.
		name := regexp.MustCompile(`"schemas":\{"([A-Za-z0-9]+)"`).FindStringSubmatch(string(spec))
		require.NotNil(t, name, "the spec must declare component schemas")
		require.Equal(t, http.StatusOK, getStatus(t, prefix+"/schemas/"+name[1]+".json"),
			"a deployment-prefixed schema document must be reachable")
	})

	t.Run("bare docs path redirects to the trailing-slash form", func(t *testing.T) {
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Get(prefix + "/api-doc")
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
		require.Equal(t, "/verifier/testxrp/sgb/api-doc/", resp.Header.Get("Location"))
	})

	t.Run("old root paths are gone", func(t *testing.T) {
		for _, path := range []string{"/api-doc", "/api-doc/", "/openapi.json", "/api/health"} {
			require.Equal(t, http.StatusNotFound, getStatus(t, base+path), path)
		}
	})

	t.Run("health look-alikes are not exempt from auth", func(t *testing.T) {
		for _, path := range []string{"/api/healthz", "/api/health/", "/api/health/extra"} {
			require.NotEqual(t, http.StatusOK, getStatus(t, prefix+path), path)
		}
	})

	t.Run("attestation endpoint without a key returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, prefix+"/PMWMultisigAccountConfigured/verify", bytes.NewBufferString(`{}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
