package paymentservice

import (
	"fmt"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:                                 "http://127.0.0.1:8545",
	TeeWalletManagerContractAddress:        "0xD036a8F254ef782cb93af4F829A1568E992c3864",
	TeeWalletProjectManagerContractAddress: "0x26d1E94963C8b382Ad66320826399E4B30347404",
	DatabaseURL:                            "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
	CChainDatabaseURL:                      "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	AttestationType:                        connector.PMWPaymentStatus,
	SourceID:                               "XRP",
}

// TODO Refactor verify s.t. we can run this without flare node
func TestPMWPaymentStatus(t *testing.T) {
	t.Skip()
	service, err := NewPaymentService(envConfig)
	require.NoError(t, err)

	verifier := service.GetVerifier()
	response, err := verifier.Verify(t.Context(), connector.IPMWPaymentStatusRequestBody{
		WalletId: common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"),
		Nonce:    10110067,
		SubNonce: 10110067,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	fmt.Println(response.SenderAddress, response.RecipientAddress)

	// https://testnet.xrpl.org/transactions/6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8/raw
	require.Equal(t, crypto.Keccak256Hash([]byte("rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK")), common.HexToHash(response.SenderAddress))
	require.Equal(t, crypto.Keccak256Hash([]byte("rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G")), common.HexToHash(response.RecipientAddress))
	require.Equal(t, big.NewInt(10_000), response.Amount)
	require.Equal(t, big.NewInt(10_000), response.ReceivedAmount)
	require.Equal(t, big.NewInt(100), response.Fee)
	require.Equal(t, big.NewInt(100), response.TransactionFee)
	require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.PaymentReference[:]))
	require.Equal(t, uint8(0), response.TransactionStatus)
	require.Equal(t, "", response.RevertReason)
	require.Equal(t, common.HexToHash("0x6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8"), common.BytesToHash(response.TransactionId[:]))
	require.Equal(t, uint64(10110073), response.BlockNumber)
}

func TestPMWPaymentStatus2(t *testing.T) {
	service, err := NewPaymentService(envConfig)
	require.NoError(t, err)

	verifier := service.GetVerifier()
	response, err := verifier.Verify(t.Context(), connector.IPMWPaymentStatusRequestBody{
		WalletId: common.HexToHash("0xeeaeab68cd8f1c11a23158ff4d475ecc5395623283da609580fe2921a34e04ea"),
		Nonce:    10140297,
		SubNonce: 10140297,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	fmt.Println(response.SenderAddress, response.RecipientAddress)

	// https://testnet.xrpl.org/transactions/6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8/raw
	//require.Equal(t, crypto.Keccak256Hash([]byte("rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK")), common.HexToHash(response.SenderAddress))
	//require.Equal(t, crypto.Keccak256Hash([]byte("rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G")), common.HexToHash(response.RecipientAddress))
	//require.Equal(t, big.NewInt(10_000), response.Amount)
	//require.Equal(t, big.NewInt(10_000), response.ReceivedAmount)
	//require.Equal(t, big.NewInt(100), response.Fee)
	//require.Equal(t, big.NewInt(100), response.TransactionFee)
	//require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.PaymentReference[:]))
	//require.Equal(t, uint8(0), response.TransactionStatus)
	//require.Equal(t, "", response.RevertReason)
	//require.Equal(t, common.HexToHash("0x6A9F06287D5CC81A6EB35B5198898701A9BE3CCF658177A0BC6A9609D06F73C8"), common.BytesToHash(response.TransactionId[:]))
	//require.Equal(t, uint64(10110073), response.BlockNumber)

	c, err := config.LoadEncodedAndAbi(config.EnvConfig{
		RPCURL:                                 "http://localhost:8545",
		RelayContractAddress:                   "0x5A0773Ff307Bf7C71a832dBB5312237fD3437f9F",
		TeeMachineRegistryContractAddress:      "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		TeeWalletManagerContractAddress:        "0xD036a8F254ef782cb93af4F829A1568E992c3864",
		TeeWalletProjectManagerContractAddress: "0x26d1E94963C8b382Ad66320826399E4B30347404",
		DatabaseURL:                            "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
		CChainDatabaseURL:                      "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
		Env:                                    "development",
		Port:                                   "123",
		ApiKeys:                                []string{"12345"},
		AttestationType:                        connector.PMWPaymentStatus,
		SourceID:                               "XRP",
	})

	result, bytes, err := handleVerifierResult[connector.IPMWPaymentStatusResponseBody](err, response, &c)
	_ = result
	require.NoError(t, err)
	newResponse := types.NewResponse(types.EncodedResponseBody{
		Response: bytes,
	})

	var request connector.IPMWPaymentStatusResponseBody
	err = structs.DecodeTo(connector.AttestationTypeArguments[connector.PMWPaymentStatus].Response, newResponse.Body.Response, &request)
	require.NoError(t, err)
}

func TestB(t *testing.T) {
	structs.Encode(connector.AttestationTypeArguments[connector.PMWPaymentStatus].Request, connector.IPMWPaymentStatusRequestBody{
		WalletId: common.HexToHash("0xeeaeab68cd8f1c11a23158ff4d475ecc5395623283da609580fe2921a34e04ea"),
		Nonce:    123,
		SubNonce: 123,
	})
}

func handleVerifierResult[T any](verifierErr error, responseData T, config *config.EncodedAndAbi) (T, []byte, error) {
	responseDataBytes, verifierErr := utils.AbiEncodeData[T](responseData, config.AbiPair.Response)
	return responseData, responseDataBytes, nil
}
