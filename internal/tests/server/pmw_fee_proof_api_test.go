package server_test

import (
	"math/big"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	"github.com/flare-foundation/go-verifier-api/internal/tests/server"
	"github.com/stretchr/testify/require"
)

// Docker-dependent test: Requires Docker services.
// See README.md, section "Running specific tests manually" for details.
func TestPMWFeeProof(t *testing.T) {
	config.ClearPMWFeeProofConfigForTest()

	setup := server.SetupServer(t, connector.PMWFeeProof, config.SourceTestXRP, config.EnvConfig{
		SourceDatabaseURL: "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	})
	defer setup.Stop()

	opType, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)

	testSenderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	// Nonces baseNonce+30..+32 = 11263175..11263177
	fromNonce := uint64(11263175)
	toNonce := uint64(11263177)
	untilTimestamp := uint64(1759820500) // After all event timestamps

	baseReqBody := connector.IPMWFeeProofRequestBody{
		OpType:         opType,
		SenderAddress:  testSenderAddress,
		FromNonce:      fromNonce,
		ToNonce:        toNonce,
		UntilTimestamp: untilTimestamp,
	}

	// Expected values from seed data:
	// Pay MaxFees: 100 + 200 + 150 = 450
	// Reissue residual: max(0, 250 - 100) = 150
	// estimatedFee = 450 + 150 = 600
	// actualFee = 12 + 15 + 10 = 37
	expectedEstimatedFee := big.NewInt(600)
	expectedActualFee := big.NewInt(37)

	desiredURL := setup.URL + "/prepareRequestBody"
	t.Run("prepareRequestBody: valid", func(t *testing.T) {
		reqData := helpers.PMWFeeProofRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := helpers.Post[types.AttestationRequest](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		internalData, err := reqData.ToInternal()
		require.NoError(t, err)

		attBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, internalData)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})
	t.Run("prepareRequestBody: invalid sourceID", func(t *testing.T) {
		reqData := helpers.PMWFeeProofRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqData)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed")
	})

	desiredURL = setup.URL + "/prepareResponseBody"
	t.Run("prepareResponseBody: valid", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := helpers.Post[types.AttestationResponseData[types.PMWFeeProofResponseBody]](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		require.Equal(t, expectedActualFee, response.ResponseData.ActualFee.ToInt())
		require.Equal(t, expectedEstimatedFee, response.ResponseData.EstimatedFee.ToInt())
	})
	t.Run("prepareResponseBody: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed")
	})
	t.Run("prepareResponseBody: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed")
	})

	desiredURL = setup.URL + "/verify"
	t.Run("verify: valid", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)

		result := helpers.DecodeResponseBody[connector.IPMWFeeProofResponseBody](t, connector.PMWFeeProof, response.ResponseBody)
		require.Equal(t, expectedActualFee, result.ActualFee)
		require.Equal(t, expectedEstimatedFee, result.EstimatedFee)
	})
	t.Run("verify: missing api-key", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, "") //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnauthorized, "Unauthorized")
	})
	t.Run("verify: wrong api-key", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, "wrong api key") //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnauthorized, "Unauthorized")
	})
	t.Run("verify: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed")
	})
	t.Run("verify: invalid attestationType", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, baseReqBody)
		request := helpers.CreateAttestationRequest(t, common.HexToHash("0x123"), setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed")
	})
	t.Run("verify: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed")
	})
	t.Run("verify: nonce range too large", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.FromNonce = 1
		modifiedReqBody.ToNonce = 102 // 102 nonces > MaxNonceRange
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Verification failed")
	})
	t.Run("verify: missing pay event", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.FromNonce = 99999 // Nonce with no pay event
		modifiedReqBody.ToNonce = 99999
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed")
	})
	t.Run("verify: missing XRP transaction", func(t *testing.T) { // Log 40: pay event exists but no XRP tx
		modifiedReqBody := baseReqBody
		modifiedReqBody.FromNonce = 11263185 // baseNonce + 40
		modifiedReqBody.ToNonce = 11263185
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed")
	})
	t.Run("verify: cannot decode event data (ABI unpack)", func(t *testing.T) { // Log 41: short data
		modifiedReqBody := baseReqBody
		modifiedReqBody.FromNonce = 11263186 // baseNonce + 41
		modifiedReqBody.ToNonce = 11263186
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed")
	})
	t.Run("verify: cannot parse XRP transaction fee", func(t *testing.T) { // Log 42: valid event, bad Fee in XRP tx
		modifiedReqBody := baseReqBody
		modifiedReqBody.FromNonce = 11263187 // baseNonce + 42
		modifiedReqBody.ToNonce = 11263187
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed")
	})
	t.Run("verify: cannot decode event data message", func(t *testing.T) { // Log 43: corrupt message encoding
		modifiedReqBody := baseReqBody
		modifiedReqBody.FromNonce = 11263188 // baseNonce + 43
		modifiedReqBody.ToNonce = 11263188
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed")
	})
	t.Run("prepareResponseBody: verification failed", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.FromNonce = 99999
		modifiedReqBody.ToNonce = 99999
		reqBody := helpers.EncodeRequestBody(t, connector.PMWFeeProof, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.PostWithoutMarshalling(t, setup.URL+"/prepareResponseBody", request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed")
	})

	config.ClearPMWFeeProofConfigForTest()
}
