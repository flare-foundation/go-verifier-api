package api_test

import (
	"fmt"
	"math/big"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	api "github.com/flare-foundation/go-verifier-api/internal/api_test"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestPMWPaymentStatus(t *testing.T) {
	const port = 3121
	const apiKey = "test-api-key"

	url, attestationType, sourceID, stop := api.SetupServer(t, connector.PMWPaymentStatus, config.SourceXRP, config.EnvConfig{
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

	opType, err := coreutil.StringToBytes32(string(config.SourceXRP))
	require.NoError(t, err)

	// /prepareRequestBody

	t.Run("prepareRequestBody: Valid request", func(t *testing.T) {
		reqData := testhelper.PMWPaymentStatusRequestBody(opType, "address", 1, 1)
		request := testhelper.CreateAttestationRequestData[types.PMWPaymentStatusRequestBody](t, attestationType, sourceID, reqData)

		response, err := testhelper.Post[types.AttestationRequest](t, fmt.Sprintf("%s/prepareRequestBody", url), request, apiKey)
		attBody := testhelper.EncodedIPMWPaymentStatusRequestBody(t, opType, reqData.SenderAddress, request.RequestData.Nonce, request.RequestData.SubNonce)

		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody: Bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.AttestationRequestData[types.PMWPaymentStatusRequestBody]{}, apiKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	// /prepareResponseBody

	t.Run("prepareResponseBody: Valid payment", func(t *testing.T) {
		t.Skip() // TODO need to update c-chain due to SC changes
		reqBody := testhelper.EncodedIPMWPaymentStatusRequestBody(t, opType, "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK", 10110067, 10110067)
		request := testhelper.CreateAttestationRequest(t, attestationType, sourceID, reqBody)

		response, err := testhelper.Post[types.AttestationResponseData[types.PMWPaymentStatusResponseBody]](t, fmt.Sprintf("%s/prepareResponseBody", url), request, apiKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		// https://testnet.xrpl.org/transactions/6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8/raw
		require.Equal(t, crypto.Keccak256Hash([]byte("rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G")), common.HexToHash(response.ResponseData.RecipientAddress))
		require.Equal(t, [32]byte{}, response.ResponseData.TokenId)
		require.Equal(t, big.NewInt(10_000), response.ResponseData.Amount.ToInt())
		require.Equal(t, big.NewInt(10_000), response.ResponseData.ReceivedAmount.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.Fee.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.TransactionFee.ToInt())
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.ResponseData.PaymentReference[:]))
		require.Equal(t, uint8(0), response.ResponseData.TransactionStatus)
		require.Equal(t, "", response.ResponseData.RevertReason)
		require.Equal(t, common.HexToHash("0x6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8"), common.BytesToHash(response.ResponseData.TransactionID[:]))
		require.Equal(t, uint64(10110073), response.ResponseData.BlockNumber)
	})

	t.Run("prepareResponseBody: Bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareResponseBody", url), types.AttestationRequest{}, apiKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("prepareResponseBody: Invalid body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, attestationType, sourceID, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareResponseBody", url), request, apiKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	// /verify

	t.Run("verify: Valid payment", func(t *testing.T) {
		t.Skip() // TODO need to update c-chain due to SC changes
		reqBody := testhelper.EncodedIPMWPaymentStatusRequestBody(t, opType, "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK", 10110067, 10110067)
		request := testhelper.CreateAttestationRequest(t, attestationType, sourceID, reqBody)
		response, err := testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", url), request, apiKey)
		require.NoError(t, err)

		result := testhelper.DecodeFTDCPMVPaymentStatusResponse(t, response.ResponseBody)
		// https://testnet.xrpl.org/transactions/6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8/raw
		require.Equal(t, crypto.Keccak256Hash([]byte("rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G")), common.HexToHash(result.RecipientAddress))
		require.Equal(t, [32]byte{}, result.TokenId)
		require.Equal(t, big.NewInt(10_000), result.Amount)
		require.Equal(t, big.NewInt(10_000), result.ReceivedAmount)
		require.Equal(t, big.NewInt(100), result.Fee)
		require.Equal(t, big.NewInt(100), result.TransactionFee)
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(result.PaymentReference[:]))
		require.Equal(t, uint8(0), result.TransactionStatus)
		require.Equal(t, "", result.RevertReason)
		require.Equal(t, common.HexToHash("0x6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8"), common.BytesToHash(result.TransactionId[:]))
		require.Equal(t, uint64(10110073), result.BlockNumber)
	})

	t.Run("verify: Missing api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, attestationType, sourceID, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), request, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})

	t.Run("verify: Wrong api-key", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, attestationType, sourceID, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), request, "wrong api key")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})

	t.Run("verify: Invalid sourceID", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, attestationType, common.HexToHash("0x123"), []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), request, apiKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("verify: Invalid attestationType", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, common.HexToHash("0x123"), sourceID, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), request, apiKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("verify: Invalid body", func(t *testing.T) {
		request := testhelper.CreateAttestationRequest(t, attestationType, sourceID, []byte("0x123"))
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), request, apiKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
}
