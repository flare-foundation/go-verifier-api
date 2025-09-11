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
	utils "github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"

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
		APIKeys:         []string{testAPIKey},
		Env:             "development",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	t.Run("PMWMultisigAccountConfigured: Health check", func(t *testing.T) {
		resp, err := testhelper.Get(t, "http://localhost:3120/api/health", testAPIKey)
		require.NoError(t, err)

		var response types.HealthCheckResponse
		require.NoError(t, json.Unmarshal(resp, &response))
		require.True(t, response.Healthy)
	})

	t.Run("PMWMultisigAccountConfigured: Test correctly created multisig wallet", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)

		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		attestationRequest, err := testhelper.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
			PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
			Threshold:     1,
		})
		require.NoError(t, err)

		response, err := testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", url), types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}, testAPIKey)
		require.NoError(t, err)

		result, err := testhelper.DecodeFTDCTeeAvailabilityCheckResponse(response.ResponseBody)
		require.NoError(t, err)

		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), result.Status)
		require.Equal(t, uint64(10136106), result.Sequence)
	})

	t.Run("PMWMultisigAccountConfigured: Test invalid sourceId", func(t *testing.T) {
		attestationRequest, err := testhelper.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
			PublicKeys:    [][]byte{},
			Threshold:     1,
		})
		require.NoError(t, err)

		attestationType, _ := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        common.HexToHash("0x123123"),
			RequestBody:     attestationRequest,
		}, testAPIKey)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("PMWMultisigAccountConfigured: Test prepareRequestBody", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		request := types.AttestationRequestData[types.PMWMultisigAccountConfiguredRequestBody]{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestData: types.PMWMultisigAccountConfiguredRequestBody{
				AccountAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
				PublicKeys:     []hexutil.Bytes{pubkey1, pubkey2, pubkey3},
				Threshold:      1,
			},
		}

		response, err := testhelper.Post[types.AttestationRequestEncoded](t, fmt.Sprintf("%s/prepareRequestBody", url), request, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.RequestBody)

		attestationRequest, err := testhelper.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: request.RequestData.AccountAddress,
			PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
			Threshold:     request.RequestData.Threshold,
		})
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attestationRequest)
	})

	t.Run("PMWMultisigAccountConfigured: Test prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.AttestationRequestData[types.PMWMultisigAccountConfiguredRequestBody]{}, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("PMWMultisigAccountConfigured: Test prepareRequestBody - empty public key", func(t *testing.T) {
		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		request := types.AttestationRequestData[types.PMWMultisigAccountConfiguredRequestBody]{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestData: types.PMWMultisigAccountConfiguredRequestBody{
				AccountAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
				PublicKeys:     []hexutil.Bytes{hexutil.Bytes{}},
				Threshold:      1,
			},
		}

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), request, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)

	})

	t.Run("PMWMultisigAccountConfigured: Test prepareResponseBody", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		attestationRequest, err := testhelper.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
			PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
			Threshold:     1,
		})
		require.NoError(t, err)

		request := types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}

		response, err := testhelper.Post[types.AttestationResponseData[types.PMWMultisigAccountConfiguredResponseBody]](t, fmt.Sprintf("%s/prepareResponseBody", url), request, testAPIKey)
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
		CChainDatabaseURL: "username:password@tcp(127.0.0.1:3306)/db?parseTime=true",
		SourceID:          config.SourceXRP,
		AttestationType:   connector.PMWPaymentStatus,
		Port:              "3121",
		APIKeys:           []string{testAPIKey},
		Env:               "development",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	t.Run("PMWPaymentStatus: Test valid payment", func(t *testing.T) {
		attestationRequest, err := testhelper.EncodeFTDCPMVPaymentStatusRequest(connector.IPMWPaymentStatusRequestBody{
			WalletId: common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"),
			Nonce:    10110067,
			SubNonce: 10110067,
		})
		require.NoError(t, err)

		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWPaymentStatus, config.SourceXRP)
		response, err := testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", url), types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}, testAPIKey)
		require.NoError(t, err)

		result, err := testhelper.DecodeFTDCPMVPaymentStatusResponse(response.ResponseBody)
		require.NoError(t, err)

		// https://testnet.xrpl.org/transactions/6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8/raw
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

	t.Run("PMWPaymentStatus: Test missing api-key", func(t *testing.T) {
		attestationType, err := utils.StringToBytes32(string(connector.PMWPaymentStatus))
		require.NoError(t, err)

		_, err = testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", url), types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        common.HexToHash("0x123"),
			RequestBody:     []byte("0x123"),
		}, "")
		require.Error(t, err)
	})

	t.Run("PMWPaymentStatus: Test wrong api-key", func(t *testing.T) {
		attestationType, err := utils.StringToBytes32(string(connector.PMWPaymentStatus))
		require.NoError(t, err)

		_, err = testhelper.Post[types.AttestationResponse](t, fmt.Sprintf("%s/verify", url), types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        common.HexToHash("0x123"),
			RequestBody:     []byte("0x123"),
		}, "wrong api key")
		require.Error(t, err)
	})

	t.Run("PMWPaymenStatus: Test invalid sourceID", func(t *testing.T) {
		attestationType, err := utils.StringToBytes32(string(connector.PMWPaymentStatus))
		require.NoError(t, err)

		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/verify", url), types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        common.HexToHash("0x123"),
			RequestBody:     []byte("0x123"),
		}, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("PMWPaymenStatus: Test prepareRequestBody", func(t *testing.T) {
		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWPaymentStatus, config.SourceXRP)

		request := types.AttestationRequestData[types.PMWPaymentStatusRequestBody]{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestData: types.PMWPaymentStatusRequestBody{
				WalletID: common.HexToHash("0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
				Nonce:    1,
				SubNonce: 1,
			},
		}
		response, err := testhelper.Post[types.AttestationRequest](t, fmt.Sprintf("%s/prepareRequestBody", url), request, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.RequestBody)

		attestationRequest, err := testhelper.EncodeFTDCPMVPaymentStatusRequest(connector.IPMWPaymentStatusRequestBody{
			WalletId: request.RequestData.WalletID,
			Nonce:    request.RequestData.Nonce,
			SubNonce: request.RequestData.SubNonce,
		})
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attestationRequest)
	})

	t.Run("PMWPaymentStatus: Test prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.AttestationRequestData[types.PMWPaymentStatusRequestBody]{}, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("PMWPaymentStatus: Test prepareResponseBody", func(t *testing.T) {
		attestationRequest, err := testhelper.EncodeFTDCPMVPaymentStatusRequest(connector.IPMWPaymentStatusRequestBody{
			WalletId: common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"),
			Nonce:    10110067,
			SubNonce: 10110067,
		})
		require.NoError(t, err)

		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWPaymentStatus, config.SourceXRP)
		request := types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}

		response, err := testhelper.Post[types.AttestationResponseData[types.PMWPaymentStatusResponseBody]](t, fmt.Sprintf("%s/prepareResponseBody", url), request, testAPIKey)
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
		require.Equal(t, common.HexToHash("0x6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8"), common.BytesToHash(response.ResponseData.TransactionID[:]))
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
		APIKeys:                           []string{testAPIKey},
		Env:                               "development",
		RelayContractAddress:              "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	t.Run("TEEAvailabilityCheck: prepareRequestBody", func(t *testing.T) {
		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.AvailabilityCheck, config.SourceTEE)
		ftdReq := types.AttestationRequestData[types.TeeAvailabilityCheckRequestBody]{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestData: types.TeeAvailabilityCheckRequestBody{
				TeeID:     common.HexToAddress("0x12345"),
				URL:       "https://example.com",
				Challenge: common.HexToHash("0x123"),
			},
		}
		response, err := testhelper.Post[types.AttestationRequestEncoded](t, fmt.Sprintf("%s/prepareRequestBody", url), ftdReq, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.RequestBody)

		attestationRequest, err := testhelper.EncodeFTDCTeeAvailabilityCheckRequest(connector.ITeeAvailabilityCheckRequestBody{
			TeeId:     ftdReq.RequestData.TeeID,
			Url:       ftdReq.RequestData.URL,
			Challenge: ftdReq.RequestData.Challenge,
		})
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attestationRequest)
	})

	t.Run("TEEAvailabilityCheck: Test prepareRequestBody - bad request", func(t *testing.T) {
		response, err := testhelper.PostWithoutMarshalling(t, fmt.Sprintf("%s/prepareRequestBody", url), types.AttestationRequestData[types.TeeAvailabilityCheckRequestBody]{}, testAPIKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("TEEAvailabilityCheck: getPolledTees", func(t *testing.T) {
		resp, err := testhelper.Get(t, "http://localhost:3122/poller/tees", testAPIKey)
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

func prepareAttestationTypeAndSourceID(t *testing.T, attestationType connector.AttestationType, sourceID config.SourceName) (common.Hash, common.Hash) {
	t.Helper()
	attestationTypeBytes, err := utils.StringToBytes32(string(attestationType))
	require.NoError(t, err)
	sourceIDBytes, err := utils.StringToBytes32(string(sourceID))
	require.NoError(t, err)
	return common.BytesToHash(attestationTypeBytes[:]), common.BytesToHash(sourceIDBytes[:])
}
