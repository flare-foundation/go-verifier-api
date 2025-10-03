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
		AllowTeeDebug:                     "false",
		DisableAttestationCheckE2E:        "false",
	})
	defer setup.Stop()
	desiredURL := fmt.Sprintf("%s/prepareRequestBody", setup.URL)
	baseReqBody := connector.ITeeAvailabilityCheckRequestBody{
		TeeId:      common.HexToAddress("0x12345"),
		TeeProxyId: common.HexToAddress("0x23456"),
		Url:        "https://example.com",
		Challenge:  common.HexToHash("0x123"),
	}
	t.Run("prepareRequestBody: valid", func(t *testing.T) {
		reqData := testhelper.TeeAvailabilityCheckRequestBody(baseReqBody)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := testhelper.Post[types.AttestationRequestEncoded](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		internalData, err := request.RequestData.ToInternal()
		require.NoError(t, err)

		attBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, internalData)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})
	t.Run("prepareRequestBody: invalid sourceID", func(t *testing.T) {
		reqData := testhelper.TeeAvailabilityCheckRequestBody(baseReqBody)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, common.HexToHash("0x12345"), reqData)

		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	desiredURL = fmt.Sprintf("%s/prepareResponseBody", setup.URL)
	t.Run("prepareResponseBody: invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("prepareResponseBody: invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("prepareResponseBody: failed verification", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
	desiredURL = fmt.Sprintf("%s/verify", setup.URL)
	t.Run("verify: invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("verify: invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("polledTees", func(t *testing.T) {
		resp, err := testhelper.Get(t, fmt.Sprintf("http://localhost:%s/poller/tees", setup.Port), setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, resp)

		var response types.TeeSamplesResponse
		require.NoError(t, json.Unmarshal(resp, &response))
		require.Empty(t, response.Samples)
	})
}
