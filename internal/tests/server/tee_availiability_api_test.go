package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	csigning "github.com/flare-foundation/go-flare-common/pkg/signing"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	"github.com/flare-foundation/go-verifier-api/internal/tests/server"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestTEEAvailabilityCheck(t *testing.T) {
	config.ClearTeeAvailabilityCheckConfigForTest()
	setup := server.SetupServer(t, fdc2.AvailabilityCheck, config.SourceTEE, config.EnvConfig{
		RPCURL:                         "https://coston-api.flare.network/ext/C/rpc",
		RelayContractAddress:           "0x92a6E1127262106611e1e129BB64B6D8654273F7",
		FlareTeeManagerContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		AllowTeeDebug:                  "true",
		DisableAttestationCheckE2E:     "true",
		AllowPrivateNetworks:           "true",
		ChainID:                        strconv.FormatUint(helpers.TestChainID, 10),
	})
	defer setup.Stop()

	contractChallenge := common.HexToHash("0x12345678901234567890")
	instructionId := common.HexToHash("0x234234234")
	privProxyKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	teeInfo, privTEEKey := helpers.TeeInfoResponse(t, contractChallenge)
	// Set up a temporary HTTP server
	handler := http.NewServeMux()
	handler.HandleFunc("/action/result/"+instructionId.Hex()[2:], func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		teeInfoBytes, err := json.Marshal(teeInfo)
		require.NoError(t, err)

		actionResult := teenodetypes.ActionResult{
			ID:        instructionId,
			Status:    1, // success for direct instructions (tee-node)
			OPType:    op.Reg.Hash(),
			OPCommand: op.TEEAttestation.Hash(),
			Data:      teeInfoBytes,
		}

		// Both signatures bind helpers.TestChainID — matching the attested
		// TeeInfo.ChainID that FetchTEEChallengeResult (proxy) and verifyActionResult
		// (TEE) reconstruct on the recovery side, and the verifier's CHAIN_ID.
		proxySignHash, err := csigning.NewPayload(csigning.ProxyActionResult, helpers.TestChainID, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		proxySignature, err := crypto.Sign(accounts.TextHash(proxySignHash[:]), privProxyKey)
		require.NoError(t, err)

		teeSignHash, err := csigning.NewPayload(csigning.TEEActionResult, helpers.TestChainID, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		teeSignature, err := crypto.Sign(accounts.TextHash(teeSignHash[:]), privTEEKey)
		require.NoError(t, err)

		resp := teenodetypes.ActionResponse{
			Result:         actionResult,
			Signature:      teeSignature,
			ProxySignature: proxySignature,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	baseReqBody := fdc2.ITeeAvailabilityCheckRequestBody{
		TeeId:         crypto.PubkeyToAddress(privTEEKey.PublicKey),
		TeeProxyId:    crypto.PubkeyToAddress(privProxyKey.PublicKey),
		Url:           server.URL,
		Challenge:     contractChallenge,
		InstructionId: instructionId,
	}
	desiredURL := setup.URL + "/prepareRequestBody"
	t.Run("prepareRequestBody: valid", func(t *testing.T) {
		reqData := helpers.TeeAvailabilityCheckRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := helpers.Post[types.AttestationRequestEncoded](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		internalData, err := request.RequestData.ToInternal()
		require.NoError(t, err)

		attBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, internalData)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})
	t.Run("prepareRequestBody: invalid sourceID", func(t *testing.T) {
		reqData := helpers.TeeAvailabilityCheckRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, common.HexToHash("0x12345"), reqData)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed")
	})
	desiredURL = setup.URL + "/prepareResponseBody"
	t.Run("prepareResponseBody: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed")
	})
	t.Run("prepareResponseBody: empty request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte{})
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "requestBody cannot be empty")
	})
	t.Run("prepareResponseBody: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed")
	})
	t.Run("prepareResponseBody: proxy ID does not match", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.TeeProxyId = common.HexToAddress("0x11")
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed")
	})
	t.Run("prepareResponseBody: valid", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.Post[types.AttestationResponseData[types.TeeAvailabilityCheckResponseBody]](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		require.Equal(t, uint8(verifier.OK), response.ResponseData.Status)
		require.Equal(t, verifier.E2ETestCodeHash[:], response.ResponseData.CodeHash[:])
		require.Equal(t, verifier.E2ETestPlatform[:], response.ResponseData.Platform[:])
	})
	desiredURL = setup.URL + "/verify"
	t.Run("verify: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed")
	})
	t.Run("verify: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed")
	})
	t.Run("verify: proxy ID does not match", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.TeeProxyId = common.HexToAddress("0x11")
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed")
	})
	t.Run("verify: challenge does not match", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.Challenge = common.HexToHash("0x11")
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed")
	})
	t.Run("verify: action result not found", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.InstructionId = common.HexToHash("0x11")
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose // test only checks status code
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusServiceUnavailable, "Verification failed")
	})
	t.Run("verify: valid", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, fdc2.AvailabilityCheck, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)

		result := helpers.DecodeResponseBody[fdc2.ITeeAvailabilityCheckResponseBody](t, fdc2.AvailabilityCheck, response.ResponseBody)
		require.NotEmpty(t, result)
		require.Equal(t, uint8(verifier.OK), result.Status)
		require.Equal(t, verifier.E2ETestCodeHash[:], result.CodeHash[:])
		require.Equal(t, verifier.E2ETestPlatform[:], result.Platform[:])
	})
}
