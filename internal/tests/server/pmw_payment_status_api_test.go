package server_test

import (
	"fmt"
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
func TestPMWPaymentStatus(t *testing.T) {
	config.ClearPMWPaymentStatusConfigForTest()

	setup := server.SetupServer(t, connector.PMWPaymentStatus, config.SourceTestXRP, config.EnvConfig{
		SourceDatabaseURL: "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	})
	defer setup.Stop()

	opType, err := convert.StringToCommonHash(string(op.XRP))
	require.NoError(t, err)

	testSenderAddress := "renoX7N3xcss6nbh62tYAhaTH1XG17Arc"
	testRecipientAddress := "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G"
	testTxHash := common.HexToHash("0x7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA")
	nonce := uint64(11263145)
	baseReqBody := connector.IPMWPaymentStatusRequestBody{
		OpType:        opType,
		SenderAddress: testSenderAddress,
		Nonce:         nonce,
		SubNonce:      nonce,
	}
	desiredURL := fmt.Sprintf("%s/prepareRequestBody", setup.URL)
	t.Run("prepareRequestBody: valid", func(t *testing.T) {
		reqData := helpers.PMWPaymentStatusRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqData)

		response, err := helpers.Post[types.AttestationRequest](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		internalData, err := reqData.ToInternal()
		require.NoError(t, err)

		attBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, internalData)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})
	t.Run("prepareRequestBody: invalid sourceID", func(t *testing.T) {
		reqData := helpers.PMWPaymentStatusRequestBody(t, baseReqBody)
		request := helpers.CreateAttestationRequestData(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqData)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})

	desiredURL = fmt.Sprintf("%s/prepareResponseBody", setup.URL)
	t.Run("prepareResponseBody: valid", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)

		response, err := helpers.Post[types.AttestationResponseData[types.PMWPaymentStatusResponseBody]](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)
		// https://testnet.xrpl.org/transactions/7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA
		require.Equal(t, testRecipientAddress, response.ResponseData.RecipientAddress)
		require.Empty(t, response.ResponseData.TokenID)
		require.Equal(t, big.NewInt(10_000), response.ResponseData.Amount.ToInt())
		require.Equal(t, big.NewInt(10_000), response.ResponseData.ReceivedAmount.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.MaxFee.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.TransactionFee.ToInt())
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.ResponseData.PaymentReference[:]))
		require.Equal(t, uint8(0), response.ResponseData.TransactionStatus)
		require.Equal(t, "", response.ResponseData.RevertReason)
		require.Equal(t, testTxHash, common.BytesToHash(response.ResponseData.TransactionID[:]))
		require.Equal(t, uint64(11263149), response.ResponseData.BlockNumber)
	})
	t.Run("prepareResponseBody: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient 5 require 32")
	})
	t.Run("prepareResponseBody: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("prepareResponseBody: verification failed", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.SenderAddress = modifiedReqBody.SenderAddress[4:] // Remove 4 for chars.
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed: cannot fetch log for instruction 0xbfc81d05ef2e4baf3c28b9da65b24c2c5403f943c0692af4c7f6bf7866f0f1ac, eventHash 0xd2b490c6cf441de1940e58ec5d773c37109f3543213cd6992247896744d8c03b: record not found")
	})
	desiredURL = fmt.Sprintf("%s/verify", setup.URL)
	t.Run("verify: valid", func(t *testing.T) { // Using log (12) in c-chain idx db and transaction 7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA from xrp idx db.
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		response, err := helpers.Post[types.AttestationResponse](t, desiredURL, request, setup.APIKey)
		require.NoError(t, err)

		result := helpers.DecodeResponseBody[connector.IPMWPaymentStatusResponseBody](t, connector.PMWPaymentStatus, response.ResponseBody)
		// https://testnet.xrpl.org/transactions/7AE054AE3A73748A4A28D31ADE4EB68E9D48DD9D22179432E7EA2E2895E459CA
		require.Equal(t, testRecipientAddress, result.RecipientAddress)
		require.Empty(t, result.TokenId)
		require.Equal(t, big.NewInt(10_000), result.Amount)
		require.Equal(t, big.NewInt(10_000), result.ReceivedAmount)
		require.Equal(t, big.NewInt(100), result.MaxFee)
		require.Equal(t, big.NewInt(100), result.TransactionFee)
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(result.PaymentReference[:]))
		require.Equal(t, uint8(0), result.TransactionStatus)
		require.Equal(t, "", result.RevertReason)
		require.Equal(t, testTxHash, common.BytesToHash(result.TransactionId[:]))
		require.Equal(t, uint64(11263149), result.BlockNumber)
	})
	t.Run("verify: missing api-key", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, "") //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnauthorized, "Unauthorized")
	})
	t.Run("verify: wrong api-key", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, "wrong api key") //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusUnauthorized, "Unauthorized")
	})
	t.Run("verify: invalid sourceID", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, common.HexToHash("0x123"), reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("verify: invalid attestationType", func(t *testing.T) {
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, baseReqBody)
		request := helpers.CreateAttestationRequest(t, common.HexToHash("0x123"), setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Request validation failed: attestation type and source id combination not supported")
	})
	t.Run("verify: invalid request body", func(t *testing.T) {
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, []byte("0x123"))
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		helpers.AssertHumaError(t, response, http.StatusBadRequest, "Decoding request body to data failed: abi: cannot marshal in to go type: length insufficient")
	})
	t.Run("verify: verification failed - not found in c-chain indexer", func(t *testing.T) {
		modifiedReqBody := baseReqBody
		modifiedReqBody.SenderAddress = modifiedReqBody.SenderAddress[4:] // Remove 4 for chars.
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed: cannot fetch log for instruction 0xbfc81d05ef2e4baf3c28b9da65b24c2c5403f943c0692af4c7f6bf7866f0f1ac, eventHash 0xd2b490c6cf441de1940e58ec5d773c37109f3543213cd6992247896744d8c03b: record not found")
	})
	t.Run("verify: verification failed - not found in xrp indexer", func(t *testing.T) { // Using fake entry log (19) in c-chain idx db.
		modifiedReqBody := baseReqBody
		modifiedReqBody.Nonce = baseReqBody.Nonce + 10
		modifiedReqBody.SubNonce = baseReqBody.SubNonce + 10
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusUnprocessableEntity, "Verification failed: cannot fetch transaction for source renoX7N3xcss6nbh62tYAhaTH1XG17Arc, nonce 11263155: record not found")
	})
	t.Run("verify: verification failed - cannot decode event data (ABI unpack)", func(t *testing.T) { // Using fake entry log (20) in c-chain idx db.
		modifiedReqBody := baseReqBody
		modifiedReqBody.Nonce = baseReqBody.Nonce + 1
		modifiedReqBody.SubNonce = baseReqBody.SubNonce + 1
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed: cannot decode event TeeInstructionsSent: ABI unpack into teeextensionregistry.TeeExtensionRegistryTeeInstructionsSent failed for event \\\"TeeInstructionsSent\\\": abi: cannot marshal in to go type: length insufficient 100 require 128")
	})
	t.Run("verify: verification failed - cannot unmarshal XRP transaction", func(t *testing.T) { // Using fake entry log (21) in c-chain idx db and fake transaction entry 7ae054ae3a73748a4a28d31ade4eb68e9d48dd9d22179432e7ea2e2895e459c3.
		modifiedReqBody := baseReqBody
		modifiedReqBody.Nonce = baseReqBody.Nonce + 2
		modifiedReqBody.SubNonce = baseReqBody.SubNonce + 2
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed: cannot unmarshal XRP transaction response: json: cannot unmarshal string into Go struct field RawTransactionData.CommonFields.Sequence of type uint")
	})
	t.Run("verify: verification failed - missing transaction result", func(t *testing.T) { // Using fake entry log (22) in c-chain idx db and fake transaction entry 7ae054ae3a73748a4a28d31ade4eb68e9d48dd9d22179432e7ea2e2895e459c5.
		modifiedReqBody := baseReqBody
		modifiedReqBody.Nonce = baseReqBody.Nonce + 3
		modifiedReqBody.SubNonce = baseReqBody.SubNonce + 3
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed: missing transaction result in raw transaction data")
	})
	t.Run("verify: verification failed - cannot build payment status response", func(t *testing.T) { // Using fake entry log (23) in c-chain idx db and fake transaction entry 7ae054ae3a73748a4a28d31ade4eb68e9d48dd9d22179432e7ea2e2895e459c6.
		modifiedReqBody := baseReqBody
		modifiedReqBody.Nonce = baseReqBody.Nonce + 4
		modifiedReqBody.SubNonce = baseReqBody.SubNonce + 4
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed: cannot build payment status response: cannot parse transaction status: transaction result too short: \\\"te\\\"")
	})
	t.Run("verify: verification failed - cannot decode event data message", func(t *testing.T) { // Using fake entry log (24) in c-chain idx db.
		modifiedReqBody := baseReqBody
		modifiedReqBody.Nonce = baseReqBody.Nonce + 5
		modifiedReqBody.SubNonce = baseReqBody.SubNonce + 5
		reqBody := helpers.EncodeRequestBody(t, connector.PMWPaymentStatus, modifiedReqBody)
		request := helpers.CreateAttestationRequest(t, setup.AttestationTypeEncoded, setup.SourceIDEncoded, reqBody)
		// The response body is closed inside AssertHumaError, so linter warning is suppressed.
		response, err := helpers.PostWithoutMarshalling(t, desiredURL, request, setup.APIKey) //nolint:bodyclose
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.StatusCode)
		helpers.AssertHumaError(t, response, http.StatusInternalServerError, "Verification failed: cannot decode TeeInstructionsSent message arguments: abi: improperly encoded uint64 value")
	})
}
