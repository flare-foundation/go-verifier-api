package paymentservice

import (
	"math/big"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/op"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:            "http://127.0.0.1:8545",
	SourceDatabaseURL: "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
	CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	AttestationType:   connector.PMWPaymentStatus,
	SourceID:          "XRP",
}

func TestPaymentService(t *testing.T) {
	t.Run("should successfully create PaymentService", func(t *testing.T) {
		service, err := NewPaymentService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.GetVerifier())
		require.NotNil(t, service.GetConfig())
	})
	t.Run("missing fields in env config", func(t *testing.T) {
		pmwpaymentstatusconfig.ClearPMWPaymentStatusConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL: "",
			CChainDatabaseURL: "",
		}
		service, err := NewPaymentService(badEnvConfig)
		require.ErrorContains(t, err, "cannot load PMWPaymentStatus config: missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL")
		require.Nil(t, service)
	})
	t.Run("using unsupported source ID", func(t *testing.T) {
		pmwpaymentstatusconfig.ClearPMWPaymentStatusConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL: "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
			CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			SourceID:          "UNSUPPORTED_SOURCE",
			AttestationType:   connector.PMWPaymentStatus,
		}
		service, err := NewPaymentService(badEnvConfig)
		require.ErrorContains(t, err, "cannot initialize PMWPaymentStatus verifier: no verifier for sourceID: UNSUPPORTED_SOURCE")
		require.Nil(t, service)
	})
	pmwpaymentstatusconfig.ClearPMWPaymentStatusConfigForTest()
}

// Should this be moved? TODO
// Both tests need docker compose running.
func TestPMWPaymentStatus(t *testing.T) {
	service, err := NewPaymentService(envConfig)
	require.NoError(t, err)

	verifier := service.GetVerifier()
	opType, err := coreutil.StringToBytes32(string(op.XRP))
	require.NoError(t, err)
	t.Run("should successfully verify PMWPaymentStatus", func(t *testing.T) {
		response, err := verifier.Verify(t.Context(), connector.IPMWPaymentStatusRequestBody{
			OpType:        opType,
			SenderAddress: "r9CWG1aj4tUsZn5agTLahfyiqnNhMhPjDt",
			Nonce:         10702286,
			SubNonce:      10702286,
		})
		require.NoError(t, err)
		require.NotNil(t, response)

		var zeroBytes32 [32]byte
		// https://testnet.xrpl.org/transactions/24671113AE7A5777AADA6A4D09903B0A2D27A6B3E55B447571BFD4845CCCE4CA
		require.Equal(t, "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G", response.RecipientAddress)
		require.Equal(t, zeroBytes32, response.TokenId)
		require.Equal(t, big.NewInt(10_000), response.Amount)
		require.Equal(t, big.NewInt(10_000), response.ReceivedAmount)
		require.Equal(t, big.NewInt(100), response.Fee)
		require.Equal(t, big.NewInt(100), response.TransactionFee)
		require.Equal(t, common.Hash{0x00, 0x01}, common.BytesToHash(response.PaymentReference[:]))
		require.Equal(t, uint8(0), response.TransactionStatus)
		require.Equal(t, "", response.RevertReason)
		require.Equal(t, common.HexToHash("0x24671113AE7A5777AADA6A4D09903B0A2D27A6B3E55B447571BFD4845CCCE4CA"), common.BytesToHash(response.TransactionId[:]))
		require.Equal(t, uint64(10702291), response.BlockNumber)
	})
	t.Run("should return error if transaction not found", func(t *testing.T) {
		service, err := NewPaymentService(envConfig)
		require.NoError(t, err)
		verifier := service.GetVerifier()
		val, err := verifier.Verify(t.Context(), connector.IPMWPaymentStatusRequestBody{
			OpType:        opType,
			SenderAddress: "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
			Nonce:         10110068,
			SubNonce:      10110068,
		})
		expected := connector.IPMWPaymentStatusResponseBody{}
		require.ErrorContains(t, err, "log not found for instruction")
		require.Equal(t, expected, val)
	})
}
