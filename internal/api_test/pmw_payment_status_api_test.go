package api_test

import (
	"fmt"
	"math/big"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	api "github.com/flare-foundation/go-verifier-api/internal/api_test"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestPMWPaymentStatus(t *testing.T) {
	setup := api.SetupServer(t, connector.PMWPaymentStatus, config.SourceTestXRP, config.EnvConfig{
		SourceDatabaseURL: "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	})
	defer setup.Stop()

	opType, err := coreutil.StringToBytes32(string(op.XRP))
	require.NoError(t, err)

	testAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	nonce := uint64(11263145)
	baseReqBody := connector.IPMWPaymentStatusRequestBody{
		OpType:        opType,
		SenderAddress: testAddress,
		Nonce:         nonce,
		SubNonce:      nonce,
	}
	desiredURL := fmt.Sprintf("%s/prepareRequestBody", setup.URL)
	t.Run("prepareRequestBody: valid", func(t *testing.T) {
		reqData := testhelper.PMWPaymentStatusRequestBody(baseReqBody)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := testhelper.Post[types.AttestationRequest](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		internalData, err := reqData.ToInternal()
		require.NoError(t, err)

		attBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, internalData)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})
	t.Run("prepareRequestBody: invalid sourceID", func(t *testing.T) {
		reqData := testhelper.PMWPaymentStatusRequestBody(baseReqBody)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqData)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})

	desiredURL = fmt.Sprintf("%s/prepareResponseBody", setup.URL)
	t.Run("prepareResponseBody: valid", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := testhelper.Post[types.AttestationResponseData[types.PMWPaymentStatusResponseBody]](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		// https://testnet.xrpl.org/transactions/7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA
		require.Equal(t, "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G", response.ResponseData.RecipientAddress)
		require.Equal(t, common.Hash{}, response.ResponseData.TokenID)
		require.Equal(t, big.NewInt(10_000), response.ResponseData.Amount.ToInt())
		require.Equal(t, big.NewInt(10_000), response.ResponseData.ReceivedAmount.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.Fee.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.TransactionFee.ToInt())
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.ResponseData.PaymentReference[:]))
		require.Equal(t, uint8(0), response.ResponseData.TransactionStatus)
		require.Equal(t, "", response.ResponseData.RevertReason)
		require.Equal(t, common.HexToHash("0x7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA"), common.BytesToHash(response.ResponseData.TransactionID[:]))
		require.Equal(t, uint64(11263149), response.ResponseData.BlockNumber)
	})
	t.Run("prepareResponseBody: invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient 5 require 32")
	})
	t.Run("prepareResponseBody: invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("prepareResponseBody: verification failed", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.SenderAddress = modifiedReqBody.SenderAddress[4:] // Remove 4 for chars.
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed: log not found for instruction 0xbfc81d05ef2e4baf3c28b9da65b24c2c5403f943c0692af4c7f6bf7866f0f1ac, eventHash 0xd2b490c6cf441de1940e58ec5d773c37109f3543213cd6992247896744d8c03b")
	})
	desiredURL = fmt.Sprintf("%s/verify", setup.URL)
	t.Run("verify: valid", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)

		result := testhelper.DecodeResponseBody[connector.IPMWPaymentStatusResponseBody](t, connector.PMWPaymentStatus, response.ResponseBody)
		// https://testnet.xrpl.org/transactions/7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA
		require.Equal(t, "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G", result.RecipientAddress)
		require.Equal(t, [32]byte{}, result.TokenId)
		require.Equal(t, big.NewInt(10_000), result.Amount)
		require.Equal(t, big.NewInt(10_000), result.ReceivedAmount)
		require.Equal(t, big.NewInt(100), result.Fee)
		require.Equal(t, big.NewInt(100), result.TransactionFee)
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(result.PaymentReference[:]))
		require.Equal(t, uint8(0), result.TransactionStatus)
		require.Equal(t, "", result.RevertReason)
		require.Equal(t, common.HexToHash("0x7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA"), common.BytesToHash(result.TransactionId[:]))
		require.Equal(t, uint64(11263149), result.BlockNumber)
	})
	t.Run("verify: missing api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, "")
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusUnauthorized, "Unauthorized")
	})
	t.Run("verify: wrong api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, "wrong api key")
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusUnauthorized, "Unauthorized")
	})
	t.Run("verify: invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("verify: invalid attestationType", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, common.HexToHash("0x123"), setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("verify: invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		testhelper.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient")
	})
	t.Run("verify: verification failed", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.SenderAddress = modifiedReqBody.SenderAddress[4:] // Remove 4 for chars.
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		testhelper.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed: log not found for instruction 0xbfc81d05ef2e4baf3c28b9da65b24c2c5403f943c0692af4c7f6bf7866f0f1ac, eventHash 0xd2b490c6cf441de1940e58ec5d773c37109f3543213cd6992247896744d8c03b")
	})
}
