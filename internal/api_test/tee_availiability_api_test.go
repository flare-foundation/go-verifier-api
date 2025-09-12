package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	api "github.com/flare-foundation/go-verifier-api/internal/api_test"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestTEEAvailabilityCheck(t *testing.T) {
	const port = 3122
	const apiKey = "test-api-key"

	url, attestationType, sourceID, stop := api.SetupServer(t, connector.AvailabilityCheck, config.SourceTEE, config.EnvConfig{
		RPCURL:                            "https://s.altnet.rippletest.net:51234",
		Port:                              fmt.Sprintf("%d", port),
		APIKeys:                           []string{apiKey},
		Env:                               "development",
		RelayContractAddress:              "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		DatabaseURL:                       "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL:                 "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	})
	defer stop()

	t.Run("prepareRequestBody", func(t *testing.T) {
		reqData := testhelper.TeeAvailabilityCheckRequestBody(common.HexToAddress("0x12345"), "https://example.com", common.HexToHash("0x123"))
		request := testhelper.CreateAttestationRequestData(t, attestationType, sourceID, reqData)

		response, err := testhelper.Post[types.AttestationRequestEncoded](t, fmt.Sprintf("%s/prepareRequestBody", url), request, apiKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		attBody := testhelper.EncodedITeeAvailabilityCheckRequestBody(t, request.RequestData.TeeID, request.RequestData.URL, request.RequestData.Challenge)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.AttestationRequestData[types.TeeAvailabilityCheckRequestBody]{}, apiKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("getPolledTees", func(t *testing.T) {
		resp, err := testhelper.Get(t, "http://localhost:3122/poller/tees", apiKey)
		require.NoError(t, err)
		require.NotEmpty(t, resp)

		var response types.TeeSamplesResponse
		require.NoError(t, json.Unmarshal(resp, &response))
		require.Empty(t, response.Samples)
	})
}
