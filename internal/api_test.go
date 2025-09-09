package main

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	testutil "github.com/flare-foundation/go-verifier-api/internal/test_util"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/api"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test"

func TestPMWMultisig(t *testing.T) {
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
		resp, err := testutil.Get(t, "http://localhost:3120/api/health", testAPIKey)
		require.NoError(t, err)

		var response types.HealthCheckResponse
		require.NoError(t, json.Unmarshal(resp, &response))
		require.True(t, response.Healthy)
	})

	t.Run("PMWMultisigAccountConfigured: Test correctly created multisig wallet", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)

		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		attestationRequest, err := testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
			PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
			Threshold:     1,
		})
		require.NoError(t, err)

		response, err := testutil.Post[types.AttestationResponse](t, "http://localhost:3120/verifier/xrp/PMWMultisigAccountConfigured/verify", types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}, testAPIKey)
		require.NoError(t, err)

		result, err := testutil.DecodeFTDCTeeAvailabilityCheckResponse(response.ResponseBody)
		require.NoError(t, err)

		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), result.Status)
		require.Equal(t, uint64(10136106), result.Sequence)
	})

	t.Run("PMWMultisigAccountConfigured: Test invalid sourceID", func(t *testing.T) {
		pubkey1, err := hexutil.Decode("0x51003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240")
		require.NoError(t, err)
		pubkey2, err := hexutil.Decode("0x06276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f")
		require.NoError(t, err)

		attestationRequest, err := testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
			PublicKeys:    [][]byte{pubkey1, pubkey2},
			Threshold:     1,
		})
		require.NoError(t, err)

		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		response, err := testutil.Post[types.AttestationResponse](t, "http://localhost:3120/verifier/xrp/PMWMultisigAccountConfigured/verify", types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}, testAPIKey)
		require.NoError(t, err)

		result, err := testutil.DecodeFTDCTeeAvailabilityCheckResponse(response.ResponseBody)
		require.NoError(t, err)

		require.Equal(t, uint8(types.PMWMultisigAccountStatusERROR), result.Status)
		require.Equal(t, uint64(0), result.Sequence)
	})

	t.Run("PMWMultisigAccountConfigured: Test prepareRequestBody", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		ftdcReq := types.AttestationRequestData[types.PMWMultisigAccountConfiguredRequestBody]{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestData: types.PMWMultisigAccountConfiguredRequestBody{
				AccountAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
				PublicKeys:     []hexutil.Bytes{pubkey1, pubkey2, pubkey3},
				Threshold:      1,
			},
		}

		response, err := testutil.Post[types.AttestationRequestEncoded](t, "http://localhost:3120/verifier/xrp/PMWMultisigAccountConfigured/prepareRequestBody", ftdcReq, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.RequestBody)

		attestationRequest, err := testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: ftdcReq.RequestData.AccountAddress,
			PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
			Threshold:     ftdcReq.RequestData.Threshold,
		})
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attestationRequest)
	})

	t.Run("PMWMultisigAccountConfigured: Test prepareResponseBody", func(t *testing.T) {
		pubkey1, pubkey2, pubkey3 := pubKeysForMultisig(t)
		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWMultisigAccountConfigured, config.SourceXRP)
		attestationRequest, err := testutil.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
			WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
			PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
			Threshold:     1,
		})

		request := types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}

		response, err := testutil.Post[types.AttestationResponseData[types.PMWMultisigAccountConfiguredResponseBody]](t, "http://localhost:3120/verifier/xrp/PMWMultisigAccountConfigured/prepareResponseBody", request, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.ResponseBody)
		require.NotEmpty(t, response.ResponseData)

		require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), response.ResponseData.PMWMultisigAccountStatus)
		require.Equal(t, uint64(10136106), response.ResponseData.Sequence)
	})

}

func prepareAttestationTypeAndSourceID(t *testing.T, attestationType connector.AttestationType, sourceID config.SourceName) (common.Hash, common.Hash) {
	t.Helper()
	attestationTypeBytes, err := utils.Bytes32(string(attestationType))
	require.NoError(t, err)
	sourceIdBytes, err := utils.Bytes32(string(sourceID))
	require.NoError(t, err)
	return common.BytesToHash(attestationTypeBytes[:]), common.BytesToHash(sourceIdBytes[:])
}

func TestPMWPaymentStatus(t *testing.T) {
	go api.RunServer(config.EnvConfig{
		RPCURL:            "https://s.altnet.rippletest.net:51234",
		DatabaseURL:       "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL: "ftso_user:ftso_pass@tcp(localhost:3306)/flare_ftso_indexer?parseTime=true",
		SourceID:          config.SourceXRP,
		AttestationType:   connector.PMWPaymentStatus,
		Port:              "3121",
		APIKeys:           []string{testAPIKey},
		Env:               "development",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	t.Run("PMWPaymenStatus: Test valid payment", func(t *testing.T) {
		attestationRequest, err := testutil.EncodeFTDCPMVPaymentStatusRequest(connector.IPMWPaymentStatusRequestBody{
			WalletId: common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"),
			Nonce:    10110067,
			SubNonce: 10110067,
		})
		require.NoError(t, err)

		attestationType, sourceID := prepareAttestationTypeAndSourceID(t, connector.PMWPaymentStatus, config.SourceXRP)
		response, err := testutil.Post[types.AttestationResponse](t, "http://localhost:3121/verifier/xrp/PMWPaymentStatus/verify", types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        sourceID,
			RequestBody:     attestationRequest,
		}, testAPIKey)
		require.NoError(t, err)

		result, err := testutil.DecodeFTDCPMVPaymentStatusResponse(response.ResponseBody)
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

	t.Run("PMWPaymenStatus: Test missing api-key", func(t *testing.T) {
		attestationType, err := utils.Bytes32(string(connector.PMWPaymentStatus))
		require.NoError(t, err)

		_, err = testutil.Post[types.AttestationResponse](t, "http://localhost:3121/verifier/xrp/PMWPaymentStatus/verify", types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        common.HexToHash("0x123"),
			RequestBody:     []byte("0x123"),
		}, "")
		require.Error(t, err)
	})

	t.Run("PMWPaymenStatus: Test invalid sourceID", func(t *testing.T) {
		attestationType, err := utils.Bytes32(string(connector.PMWPaymentStatus))
		require.NoError(t, err)

		_, err = testutil.Post[types.AttestationResponse](t, "http://localhost:3121/verifier/xrp/PMWPaymentStatus/verify", types.AttestationRequest{
			AttestationType: attestationType,
			SourceID:        common.HexToHash("0x123"),
			RequestBody:     []byte("0x123"),
		}, testAPIKey)
		require.Error(t, err)
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
		response, err := testutil.Post[types.AttestationRequest](t, "http://localhost:3121/verifier/xrp/PMWPaymentStatus/prepareRequestBody", request, testAPIKey)
		require.NoError(t, err)

		require.NotEmpty(t, response.RequestBody)

		attestationRequest, err := testutil.EncodeFTDCPMVPaymentStatusRequest(connector.IPMWPaymentStatusRequestBody{
			WalletId: request.RequestData.WalletID,
			Nonce:    request.RequestData.Nonce,
			SubNonce: request.RequestData.SubNonce,
		})
		require.NoError(t, err)
		require.Equal(t, []byte(response.RequestBody), attestationRequest)
	})

	t.Run("PMWPaymenStatus: Test prepareResponseBody", func(t *testing.T) {
		attestationRequest, err := testutil.EncodeFTDCPMVPaymentStatusRequest(connector.IPMWPaymentStatusRequestBody{
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

		response, err := testutil.Post[types.AttestationResponseData[types.PMWPaymentStatusResponseBody]](t, "http://localhost:3121/verifier/xrp/PMWPaymentStatus/prepareResponseBody", request, testAPIKey)
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
