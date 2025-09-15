package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	api "github.com/flare-foundation/go-verifier-api/internal/api_test"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestTEEAvailabilityCheck(t *testing.T) {
	setup := api.SetupServer(t, connector.AvailabilityCheck, config.SourceTEE, config.EnvConfig{
		RPCURL:                            "https://coston-api.flare.network/ext/C/rpc",
		RelayContractAddress:              "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
	})
	defer setup.Stop()

	t.Run("prepareRequestBody", func(t *testing.T) {
		reqData := testhelper.TeeAvailabilityCheckRequestBody(common.HexToAddress("0x12345"), common.HexToAddress("0x12345"), "https://example.com", common.HexToHash("0x123"))
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := testhelper.Post[types.AttestationRequestEncoded](t, fmt.Sprintf("%s/prepareRequestBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		attBody := testhelper.EncodedITeeAvailabilityCheckRequestBody(t, request.RequestData.TeeID, request.RequestData.TeeProxyID, request.RequestData.URL, request.RequestData.Challenge)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", setup.URL), types.AttestationRequestData[types.TeeAvailabilityCheckRequestBody]{}, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})

	t.Run("getPolledTees", func(t *testing.T) {
		resp, err := testhelper.Get(t, fmt.Sprintf("http://localhost:%s/poller/tees", setup.Port), setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, resp)

		var response types.TeeSamplesResponse
		require.NoError(t, json.Unmarshal(resp, &response))
		require.Empty(t, response.Samples)
	})
}
