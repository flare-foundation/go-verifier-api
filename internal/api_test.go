package main

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/test_util"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
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
		ApiKeys:         []string{testAPIKey},
		Env:             "development",
	})

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	pubkey1, err := hexutil.Decode("0x51003727e9d42e8be45a851c3b86386d27df8e01630f27aaf0ea254dcb6390920d7015365559f9546f3593dd48baae0120495fef2986f87873ca116c39416240")
	require.NoError(t, err)
	pubkey2, err := hexutil.Decode("0x06276df7b93cd7fdc34c95a93e3b23466ae3416ad56d59a746fc53ab4446104ac5e545cc021561ff80bd80c411006af1c0711492259894482d995a80cd6c7e8f")
	require.NoError(t, err)
	pubkey3, err := hexutil.Decode("0x76e4a85207c1012283a7190b1df628e29ba1a687404ec35a766e7eddba94ba42a07f356ccc847540b4ed23f15f3feb07c406c3f815a361983c321740fa998cdb")
	require.NoError(t, err)

	attestationRequest, err := test_util.EncodeFTDCPMWMultisigAccountConfiguredRequest(connector.IPMWMultisigAccountConfiguredRequestBody{
		WalletAddress: "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL",
		PublicKeys:    [][]byte{pubkey1, pubkey2, pubkey3},
		Threshold:     1,
	})
	require.NoError(t, err)

	attestationType, err := utils.Bytes32(string(connector.PMWMultisigAccountConfigured))
	require.NoError(t, err)
	sourceId, err := utils.Bytes32(string(config.SourceXRP))
	require.NoError(t, err)

	resp, err := test_util.MakePostRequest(t, "http://localhost:3120/verifier/xrp/PMWMultisigAccountConfigured/verify", connector.IFtdcHubFtdcAttestationRequest{
		Header: connector.IFtdcHubFtdcRequestHeader{
			AttestationType: attestationType,
			SourceId:        sourceId,
		},
		RequestBody: attestationRequest,
	}, testAPIKey)
	require.NoError(t, err)

	var response types.EncodedResponseBody
	require.NoError(t, json.Unmarshal(resp, &response))

	result, err := test_util.DecodeFTDCTeeAvailabilityCheckResponse(response.Response)
	require.NoError(t, err)

	require.Equal(t, uint8(types.PMWMultisigAccountStatusOK), result.Status)
	require.Equal(t, uint64(10136106), result.Sequence)
}

func TestPMWPaymentStatus(t *testing.T) {
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

	attestationRequest, err := test_util.EncodeFTDCPMVPaymentStatusRequest(connector.IPMWPaymentStatusRequestBody{
		WalletId: common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"),
		Nonce:    10110067,
		SubNonce: 10110067,
	})
	require.NoError(t, err)

	attestationType, err := utils.Bytes32(string(connector.PMWPaymentStatus))
	require.NoError(t, err)
	sourceId, err := utils.Bytes32(string(config.SourceXRP))
	require.NoError(t, err)

	resp, err := test_util.MakePostRequest(t, "http://localhost:3121/verifier/xrp/PMWPaymentStatus/verify", connector.IFtdcHubFtdcAttestationRequest{
		Header: connector.IFtdcHubFtdcRequestHeader{
			AttestationType: attestationType,
			SourceId:        sourceId,
		},
		RequestBody: attestationRequest,
	}, testAPIKey)
	require.NoError(t, err)

	var response types.EncodedResponseBody
	require.NoError(t, json.Unmarshal(resp, &response))

	result, err := test_util.DecodeFTDCPMVPaymentStatusResponse(response.Response)
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
}
