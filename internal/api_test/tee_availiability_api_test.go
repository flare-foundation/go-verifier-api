package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	api "github.com/flare-foundation/go-verifier-api/internal/api_test"
	teetypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestTEEAvailabilityCheck(t *testing.T) {
	setup := api.SetupServer(t, connector.AvailabilityCheck, config.SourceTEE, config.EnvConfig{
		RPCURL:                            "https://coston-api.flare.network/ext/C/rpc",
		RelayContractAddress:              "0x92a6E1127262106611e1e129BB64B6D8654273F7",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		AllowTeeDebug:                     "true",
		DisableAttestationCheckE2E:        "true",
	})
	defer setup.Stop()

	contractChallenge := common.HexToHash("0x12345678901234567890")
	instructionId := common.HexToHash("0x234234234")
	teeTimestamp := uint64(111)
	privTEEKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	privProxyKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Set up a temporary HTTP server
	handler := http.NewServeMux()
	handler.HandleFunc(fmt.Sprintf("/action/result/%s", instructionId.Hex()[2:]), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		teeInfo := testhelper.GetTeeInfoResponse(contractChallenge, privTEEKey, teeTimestamp)
		teeInfoBytes, err := json.Marshal(teeInfo)
		require.NoError(t, err)
		hash := crypto.Keccak256(teeInfoBytes)
		ethHash := accounts.TextHash(hash)
		proxySignature, err := crypto.Sign(ethHash, privProxyKey)
		require.NoError(t, err)

		resp := teenodetypes.ActionResponse{
			Result: teenodetypes.ActionResult{
				Data: teeInfoBytes,
			},
			Signature:      []byte{},
			ProxySignature: proxySignature,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	baseReqBody := connector.ITeeAvailabilityCheckRequestBody{
		TeeId:         crypto.PubkeyToAddress(privTEEKey.PublicKey),
		TeeProxyId:    crypto.PubkeyToAddress(privProxyKey.PublicKey),
		Url:           server.URL,
		Challenge:     contractChallenge,
		InstructionId: instructionId,
	}
	desiredURL := fmt.Sprintf("%s/prepareRequestBody", setup.URL)
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
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	desiredURL = fmt.Sprintf("%s/prepareResponseBody", setup.URL)
	t.Run("prepareResponseBody: invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient 5 require 32")
	})
	t.Run("prepareResponseBody: empty request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte{})
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusUnprocessableEntity, "requestBody cannot be empty")
	})
	t.Run("prepareResponseBody: invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("prepareResponseBody: valid", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.Post[types.AttestationResponseData[types.TeeAvailabilityCheckResponseBody]](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		require.Equal(t, uint8(teetypes.OK), response.ResponseData.Status)
		require.Equal(t, teeTimestamp, response.ResponseData.TeeTimestamp)
		require.Equal(t, verifier.E2ETestCodeHash[:], response.ResponseData.CodeHash[:])
		require.Equal(t, verifier.E2ETestPlatform[:], response.ResponseData.Platform[:])
	})
	desiredURL = fmt.Sprintf("%s/verify", setup.URL)
	t.Run("verify: invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient 5 require 32")
	})
	t.Run("verify: invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("verify: proxy ID does not match", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.TeeProxyId = common.HexToAddress("0x11")
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, modifiedReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusInternalServerError, fmt.Sprintf("Verification failed: proxy signer does not match: expected 0x0000000000000000000000000000000000000011, got %s", baseReqBody.TeeProxyId))
	})
	t.Run("verify: challenge does not match", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.Challenge = common.HexToHash("0x11")
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, modifiedReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusInternalServerError, fmt.Sprintf("Verification failed: challenge does not match: expected 0x0000000000000000000000000000000000000000000000000000000000000011, got %s", contractChallenge))
	})
	t.Run("verify: not enough TEE poller data", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.InstructionId = common.HexToHash("0x11")
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, modifiedReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusInternalServerError, fmt.Sprintf("Verification failed: insufficient samples to determine TEE %s status", baseReqBody.TeeId))
	})
	t.Run("verify: valid", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.AvailabilityCheck, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := testhelper.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)

		result := testhelper.DecodeResponseBody[connector.ITeeAvailabilityCheckResponseBody](t, connector.AvailabilityCheck, response.ResponseBody)
		require.NotEmpty(t, result)
		require.Equal(t, uint8(teetypes.OK), result.Status)
		require.Equal(t, teeTimestamp, result.TeeTimestamp)
		require.Equal(t, verifier.E2ETestCodeHash[:], result.CodeHash[:])
		require.Equal(t, verifier.E2ETestPlatform[:], result.Platform[:])
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
