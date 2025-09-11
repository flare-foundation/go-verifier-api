package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	testutil "github.com/flare-foundation/go-verifier-api/internal/test_util"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test"

func TestPMWMultisig(t *testing.T) {
	const url = "http://localhost:3120/verifier/xrp/PMWMultisigAccountConfigured"
	go api.RunServer(config.EnvConfig{
		RPCURL:          "https://s.altnet.rippletest.net:51234",
		SourceID:        config.SourceXRP,
		AttestationType: connector.PMWMultisigAccountConfigured,
		Port:            "3120",
		ApiKeys:         []string{testAPIKey},
		Env:             "development",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)
	attestationType, sourceId := prepareAttestationTypeAndSourceId(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)

	t.Run("Health check", func(t *testing.T) {
		resp, err := testutil.Get(t, "http://localhost:3120/api/health", testAPIKey)
		require.NoError(t, err)

		var response types.HealthCheckResponse
		require.NoError(t, json.Unmarshal(resp, &response))
		require.True(t, response.Healthy)
	})

	t.Run("verify - correctly created multisig wallet", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		attBody := testutil.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{pubkey1, pubkey2, pubkey3}, 1)
		response, err := testutil.Post[types.EncodedResponseBody](t, fmt.Sprintf("%s/verify", url), testutil.CreateIFtdcHubFtdcAttestationRequest(t, attestationType, sourceId, attBody), testAPIKey)
		require.NoError(t, err)

		result := testutil.DecodeFTDCPMWMultisigAccountConfiguredResponse(t, response.Response)
		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), result.Status)
		require.Equal(t, uint64(10136106), result.Sequence)
	})

	t.Run("verify - invalid sourceId", func(t *testing.T) {
		attBody := testutil.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{}, 1)
		response, err := testutil.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), testutil.CreateIFtdcHubFtdcAttestationRequest(t, attestationType, common.HexToHash("0x123123"), attBody), testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("prepareRequestBody", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		ftdcReq := testutil.InternalFTDCRequest(t, attestationType, sourceId,
			types.PMWMultisigAccountRequestBody{
				WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
				PublicKeys:    []hexutil.Bytes{pubkey1, pubkey2, pubkey3},
				Threshold:     1,
			},
		)

		response, err := testutil.Post[types.EncodedRequestBody](t, fmt.Sprintf("%s/prepareRequestBody", url), ftdcReq, testAPIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		attBody := testutil.EncodedIPMWMultisigAccountConfiguredRequestBody(t, ftdcReq.RequestData.WalletAddress, [][]byte{pubkey1, pubkey2, pubkey3}, ftdcReq.RequestData.Threshold)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testutil.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.PMWMultisigAccountRequest{}, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("prepareRequestBody - empty public key", func(t *testing.T) {
		ftdcReq := testutil.InternalFTDCRequest(t, attestationType, sourceId,
			types.PMWMultisigAccountRequestBody{
				WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
				PublicKeys:    []hexutil.Bytes{{}},
				Threshold:     1,
			})

		response, err := testutil.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), ftdcReq, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)

	})

	t.Run("prepareResponseBody", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		attBody := testutil.EncodedIPMWMultisigAccountConfiguredRequestBody(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", [][]byte{pubkey1, pubkey2, pubkey3}, 1)
		ftdcReq := testutil.FTDCRequestEncoded(t, attestationType, sourceId, attBody)

		response, err := testutil.Post[types.RawAndEncodedPMWMultisigAccountResponseBody](t, fmt.Sprintf("%s/prepareResponseBody", url), ftdcReq, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)

		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), response.ResponseData.PMWMultisigAccountStatus)
		require.Equal(t, uint64(10136106), response.ResponseData.Sequence)
	})
}

func TestPMWPaymentStatus(t *testing.T) {
	const url = "http://localhost:3121/verifier/xrp/PMWPaymentStatus"
	go api.RunServer(config.EnvConfig{
		RPCURL:            "https://s.altnet.rippletest.net:51234",
		DatabaseURL:       "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
		SourceID:          config.SourceXRP,
		AttestationType:   connector.PMWPaymentStatus,
		Port:              "3121",
		ApiKeys:           []string{testAPIKey},
		Env:               "development",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	attestationType, sourceId := prepareAttestationTypeAndSourceId(t, connector.PMWPaymentStatus, config.SourceXRP)

	t.Run("verify - valid payment", func(t *testing.T) {
		attBody := testutil.EncodedIPMWPaymentStatusRequestBody(t, common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"), 10110067, 10110067)
		response, err := testutil.Post[types.EncodedResponseBody](t, fmt.Sprintf("%s/verify", url), testutil.CreateIFtdcHubFtdcAttestationRequest(t, attestationType, sourceId, attBody), testAPIKey)
		require.NoError(t, err)

		// https://testnet.xrpl.org/transactions/6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8/raw
		result := testutil.DecodeFTDCPMVPaymentStatusResponse(t, response.Response)
		require.Equal(t, crypto.Keccak256Hash([]byte("rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK")), common.HexToHash(result.SenderAddress))
		require.Equal(t, crypto.Keccak256Hash([]byte("rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G")), common.HexToHash(result.RecipientAddress))
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

	t.Run("verify - missing api-key", func(t *testing.T) {
		_, err := testutil.Post[types.EncodedResponseBody](t, fmt.Sprintf("%s/verify", url), testutil.CreateIFtdcHubFtdcAttestationRequest(t, attestationType, common.HexToHash("0x123"), []byte("0x123")), testAPIKey)
		require.Error(t, err)
	})

	t.Run("verify - wrong api-key", func(t *testing.T) {
		_, err := testutil.Post[types.EncodedResponseBody](t, fmt.Sprintf("%s/verify", url), testutil.CreateIFtdcHubFtdcAttestationRequest(t, attestationType, common.HexToHash("0x123"), []byte("0x123")), "wrong api key")
		require.Error(t, err)
	})

	t.Run("verify - invalid sourceId", func(t *testing.T) {
		attestationRequest := testutil.CreateIFtdcHubFtdcAttestationRequest(t, attestationType, common.HexToHash("0x123"), []byte("0x123"))
		response, err := testutil.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), attestationRequest, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("prepareRequestBody", func(t *testing.T) {
		ftdcReq := testutil.InternalFTDCRequest(t, attestationType, sourceId, types.PMWPaymentStatusRequestBody{
			WalletId: common.HexToHash("0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			Nonce:    1,
			SubNonce: 1,
		})
		response, err := testutil.Post[types.EncodedRequestBody](t, fmt.Sprintf("%s/prepareRequestBody", url), ftdcReq, testAPIKey)
		attBody := testutil.EncodedIPMWPaymentStatusRequestBody(t, ftdcReq.RequestData.WalletId, ftdcReq.RequestData.Nonce, ftdcReq.RequestData.SubNonce)

		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testutil.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.PMWPaymentStatusRequest{}, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("prepareResponseBody  - valid payment", func(t *testing.T) {
		attBody := testutil.EncodedIPMWPaymentStatusRequestBody(t, common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"), 10110067, 10110067)
		ftdcReq := testutil.FTDCRequestEncoded(t, attestationType, sourceId, attBody)

		response, err := testutil.Post[types.RawAndEncodedPMWPaymentStatusResponseBody](t, fmt.Sprintf("%s/prepareResponseBody", url), ftdcReq, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)

		// https://testnet.xrpl.org/transactions/6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8/raw
		require.Equal(t, crypto.Keccak256Hash([]byte("rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK")), common.HexToHash(response.ResponseData.SenderAddress))
		require.Equal(t, crypto.Keccak256Hash([]byte("rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G")), common.HexToHash(response.ResponseData.RecipientAddress))
		require.Equal(t, big.NewInt(10_000), response.ResponseData.Amount.ToInt())
		require.Equal(t, big.NewInt(10_000), response.ResponseData.ReceivedAmount.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.Fee.ToInt())
		require.Equal(t, big.NewInt(100), response.ResponseData.TransactionFee.ToInt())
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.ResponseData.PaymentReference[:]))
		require.Equal(t, uint8(0), response.ResponseData.TransactionStatus)
		require.Equal(t, "", response.ResponseData.RevertReason)
		require.Equal(t, common.HexToHash("0x6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8"), common.BytesToHash(response.ResponseData.TransactionId[:]))
		require.Equal(t, uint64(10110073), response.ResponseData.BlockNumber)
	})
}

func TestTEEAvailabilityCheck(t *testing.T) {
	const url = "http://localhost:3122/verifier/tee/TeeAvailabilityCheck"
	go api.RunServer(config.EnvConfig{
		RPCURL:                            "https://example.io",
		SourceID:                          config.SourceTEE,
		AttestationType:                   connector.AvailabilityCheck,
		Port:                              "3122",
		ApiKeys:                           []string{testAPIKey},
		Env:                               "development",
		RelayContractAddress:              "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)
	attestationType, sourceId := prepareAttestationTypeAndSourceId(t, connector.AvailabilityCheck, config.SourceTEE)

	t.Run("prepareRequestBody", func(t *testing.T) {
		ftdcReq := testutil.InternalFTDCRequest(t, attestationType, sourceId, types.TeeAvailabilityRequestBody{
			TeeId:     common.HexToAddress("0x12345"),
			Url:       "https://example.com",
			Challenge: common.HexToHash("0x123"),
		})
		response, err := testutil.Post[types.EncodedRequestBody](t, fmt.Sprintf("%s/prepareRequestBody", url), ftdcReq, testAPIKey)
		require.NoError(t, err)
		require.NotEmpty(t, response.RequestBody)

		attBody := testutil.EncodedITeeAvailabilityCheckRequestBody(t, ftdcReq.RequestData.TeeId, ftdcReq.RequestData.Url, ftdcReq.RequestData.Challenge)
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attBody)
	})

	t.Run("prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testutil.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.TeeAvailabilityRequest{}, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("getPolledTees", func(t *testing.T) {
		resp, err := testutil.Get(t, "http://localhost:3122/poller/tees", testAPIKey)
		require.NoError(t, err)
		require.NotEmpty(t, resp)

		var response types.TeeSamplesResponse
		require.NoError(t, json.Unmarshal(resp, &response))
		require.Empty(t, response.Samples)
	})
}

// pubKeysForMultisig returns set of public keys for wallet rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL
func pubKeysForMultisig(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()
	pubkey1, err := hexutil.Decode("0x51003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240")
	require.NoError(t, err)
	pubkey2, err := hexutil.Decode("0x06276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f")
	require.NoError(t, err)
	pubkey3, err := hexutil.Decode("0x76e4a85207c1012283a7190b1df628e29ba1a687404ec35a766e7eddba94ba42a07f356ccc847540b4ed23f15f3feb07c406c3f815a361983c321740fa998cdb")
	require.NoError(t, err)
	return pubkey1, pubkey2, pubkey3
}

func prepareAttestationTypeAndSourceId(t *testing.T, attestationType connector.AttestationType, sourceId config.SourceName) (common.Hash, common.Hash) {
	t.Helper()
	var attestationTypeBytes, sourceIdBytes [32]byte
	copy(attestationTypeBytes[:], attestationType)
	copy(sourceIdBytes[:], sourceId)
	return common.BytesToHash(attestationTypeBytes[:]), common.BytesToHash(sourceIdBytes[:])
}
