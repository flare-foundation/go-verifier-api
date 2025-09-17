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
		DatabaseURL:       "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
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

	// /prepareRequestBody
	t.Run("prepareRequestBody: Valid request", func(t *testing.T) {
		reqData := testhelper.PMWPaymentStatusRequestBody(baseReqBody)
		request := testhelper.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := testhelper.Post[types.AttestationRequest](t, fmt.Sprintf("%s/prepareRequestBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		internalData, err := reqData.ToInternal()
		require.NoError(t, err)

		attBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, internalData)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody: Bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", setup.URL), types.AttestationRequestData[types.PMWPaymentStatusRequestBody]{}, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})

	// /prepareResponseBody
	t.Run("prepareResponseBody: Valid payment", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := testhelper.Post[types.AttestationResponseData[types.PMWPaymentStatusResponseBody]](t, fmt.Sprintf("%s/prepareResponseBody", setup.URL), request, setup.APIKey)
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

	t.Run("prepareResponseBody: Bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareResponseBody", setup.URL), types.AttestationRequest{}, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
	})

	t.Run("prepareResponseBody: Invalid body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareResponseBody", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	// /verify
	t.Run("verify: Valid payment", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
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

	t.Run("verify: Missing api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})

	t.Run("verify: Wrong api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, "wrong api key")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})

	t.Run("verify: Invalid sourceID", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("verify: Invalid attestationType", func(t *testing.T) {
		reqBody := testhelper.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := testhelper.CreateAttestationRequest(t, common.HexToHash("0x123"), setup.SourceIDEncoded, reqBody)
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("verify: Invalid body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", setup.URL), request, setup.APIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
}
