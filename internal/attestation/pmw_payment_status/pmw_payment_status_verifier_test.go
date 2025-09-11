package paymentservice

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:            "http://127.0.0.1:8545",
	DatabaseURL:       "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
	CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	AttestationType:   connector.PMWPaymentStatus,
	SourceID:          "XRP",
}

func TestPaymentService(t *testing.T) {
	t.Run("Should successfully create PaymentService", func(t *testing.T) {
		service, err := NewPaymentService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.GetVerifier())
		require.NotNil(t, service.GetConfig())
	})

	t.Run("Missing fields in env config", func(t *testing.T) {
		pmwpaymentstatusconfig.ClearPMWPaymentStatusConfigForTest()
		badEnvConfig := config.EnvConfig{
			DatabaseURL:       "",
			CChainDatabaseURL: "",
		}
		service, err := NewPaymentService(badEnvConfig)
		require.Error(t, err)
		require.Nil(t, service)
	})

	t.Run("Using unsupported source ID", func(t *testing.T) {
		pmwpaymentstatusconfig.ClearPMWPaymentStatusConfigForTest()
		badEnvConfig := config.EnvConfig{
			DatabaseURL:       "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
			CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			SourceID:          "UNSUPPORTED_SOURCE",
			AttestationType:   connector.PMWPaymentStatus,
		}
		service, err := NewPaymentService(badEnvConfig)
		require.Error(t, err)
		require.Nil(t, service)
	})

	pmwpaymentstatusconfig.ClearPMWPaymentStatusConfigForTest()
}

// Both tests need docker compose running.
func TestPMWPaymentStatus(t *testing.T) {
	service, err := NewPaymentService(envConfig)
	require.NoError(t, err)

	verifier := service.GetVerifier()
	t.Run("Should successfully verify PMWPaymentStatus", func(t *testing.T) {
		response, err := verifier.Verify(t.Context(), connector.IPMWPaymentStatusRequestBody{
			WalletId: common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"),
			Nonce:    10110067,
			SubNonce: 10110067,
		})
		require.NoError(t, err)
		require.NotNil(t, response)

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
	})

	t.Run("Should return error if transaction not found", func(t *testing.T) {
		service, err := NewPaymentService(envConfig)
		require.NoError(t, err)

		verifier := service.GetVerifier()
		_, err = verifier.Verify(t.Context(), connector.IPMWPaymentStatusRequestBody{
			WalletId: common.HexToHash("0x4e6f4d9d6229527708f88445218fb57579c925723b13541a78ecbe31df5d2fab"),
			Nonce:    10110068,
			SubNonce: 10110068,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "log not found for instruction")
	})

}
