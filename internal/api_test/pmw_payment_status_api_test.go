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
	setup := api.SetupServer(t, connector.PMWPaymentStatus, config.SourceXRP, config.EnvConfig{
		SourceDatabaseURL: "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	})
	defer setup.Stop()

	opType, err := coreutil.StringToBytes32(string(op.XRP))
	require.NoError(t, err)

	testAddress := "r9CWG1aj4tUsZn5agTLahfyiqnNhMhPjDt"
	nonce := uint64(10702286)
	baseReqBody := connector.IPMWPaymentStatusRequestBody{
		OpType:        opType,
		SenderAddress: testAddress,
		Nonce:         nonce,
		SubNonce:      nonce,
	}
	desiredURL := fmt.Sprintf("%s/prepareRequestBody", setup.URL)
	t.Run("PrepareRequestBody: Valid request", func(t *testing.T) {
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
	t.Run("PrepareRequestBody: Invalid sourceID", func(t *testing.T) {
		reqData := testhelper.PMWPaymentStatusRequestBody(baseReqBody)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqData)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	desiredURL = fmt.Sprintf("%s/prepareResponseBody", setup.URL)
	t.Run("PrepareResponseBody: Valid payment", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := testhelper.Post[types.AttestationResponseData[types.PMWPaymentStatusResponseBody]](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		// https://testnet.xrpl.org/transactions/24671113AE7A5777AADA6A4D09903B0A2D27A6B3E55B447571BFD4845CCCE4CA
		require.Equal(t, "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G", response.ResponseData.RecipientAddress)
		require.Equal(t, common.Hash{}, response.ResponseData.TokenID)
		require.Equal(t, big.NewInt(10_000), response.ResponseData.Amount.ToInt())
		require.Equal(t, big.NewInt(10_000), response.ResponseData.ReceivedAmount.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.Fee.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.TransactionFee.ToInt())
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.ResponseData.PaymentReference[:]))
		require.Equal(t, uint8(0), response.ResponseData.TransactionStatus)
		require.Equal(t, "", response.ResponseData.RevertReason)
		require.Equal(t, common.HexToHash("0x24671113AE7A5777AADA6A4D09903B0A2D27A6B3E55B447571BFD4845CCCE4CA"), common.BytesToHash(response.ResponseData.TransactionID[:]))
		require.Equal(t, uint64(10702291), response.ResponseData.BlockNumber)
	})
	t.Run("PrepareResponseBody: Invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("PrepareResponseBody: Invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("PrepareResponseBody: Verification failed", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.SenderAddress = modifiedReqBody.SenderAddress[4:] // Remove 4 for chars.
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
	desiredURL = fmt.Sprintf("%s/verify", setup.URL)
	t.Run("Verify: Valid payment", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)

		result := testhelper.DecodeResponseBody[connector.IPMWPaymentStatusResponseBody](t, connector.PMWPaymentStatus, response.ResponseBody)
		// https://testnet.xrpl.org/transactions/24671113AE7A5777AADA6A4D09903B0A2D27A6B3E55B447571BFD4845CCCE4CA
		require.Equal(t, "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G", result.RecipientAddress)
		require.Equal(t, [32]byte{}, result.TokenId)
		require.Equal(t, big.NewInt(10_000), result.Amount)
		require.Equal(t, big.NewInt(10_000), result.ReceivedAmount)
		require.Equal(t, big.NewInt(100), result.Fee)
		require.Equal(t, big.NewInt(100), result.TransactionFee)
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(result.PaymentReference[:]))
		require.Equal(t, uint8(0), result.TransactionStatus)
		require.Equal(t, "", result.RevertReason)
		require.Equal(t, common.HexToHash("0x24671113AE7A5777AADA6A4D09903B0A2D27A6B3E55B447571BFD4845CCCE4CA"), common.BytesToHash(result.TransactionId[:]))
		require.Equal(t, uint64(10702291), result.BlockNumber)
	})
	t.Run("Verify: Missing api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("Verify: Wrong api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, "wrong api key")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
	t.Run("Verify: Invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("Verify: Invalid attestationType", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, common.HexToHash("0x123"), setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("Verify: Invalid request body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
	t.Run("Verify: Verification failed", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.SenderAddress = modifiedReqBody.SenderAddress[4:] // Remove 4 for chars.
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})
}
