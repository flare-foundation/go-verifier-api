package paymentservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:            "http://127.0.0.1:8545",
	SourceDatabaseURL: "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
	CChainDatabaseURL: "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	AttestationType:   connector.PMWPaymentStatus,
	SourceID:          "testXRP",
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
