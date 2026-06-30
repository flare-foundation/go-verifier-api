//go:build integration

package paymentservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:                         "http://127.0.0.1:8545",
	SourceDatabaseURL:              "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
	CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
	FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
	TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
	AttestationType:                fdc2.PMWPaymentStatus,
	SourceID:                       "testXRP",
}

// Docker-dependent test: gated behind the `integration` build tag and requires the
// DB fixtures (see README.md, "Running Tests"). Run with `go test -tags integration`.
// Non-DB constructor cases (missing config, unsupported source) live in the untagged
// service_preflight_test.go so they run in the default suite.
func TestNewPaymentService(t *testing.T) {
	t.Run("should successfully create PaymentService", func(t *testing.T) {
		service, err := NewPaymentService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.Verifier())
		require.NotNil(t, service.Config())
		// Close releases both DB connections and the verifier's on-chain RPC client.
		require.NoError(t, service.Close())
	})
	t.Run("misconfigured Source DB", func(t *testing.T) {
		config.ClearPMWPaymentStatusConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres:",
			CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
			RPCURL:                         "http://127.0.0.1:8545",
			SourceID:                       "testXRP",
			AttestationType:                fdc2.PMWPaymentStatus,
		}
		service, err := NewPaymentService(badEnvConfig)
		require.ErrorContains(t, err, "cannot connect to Source DB:")
		require.Nil(t, service)
	})
	t.Run("misconfigured CChain DB", func(t *testing.T) {
		config.ClearPMWPaymentStatusConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
			CChainDatabaseURL:              "root:root@tcp()",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
			RPCURL:                         "http://127.0.0.1:8545",
			SourceID:                       "testXRP",
			AttestationType:                fdc2.PMWPaymentStatus,
		}
		service, err := NewPaymentService(badEnvConfig)
		require.ErrorContains(t, err, "cannot connect to CChain DB:")
		require.Nil(t, service)
	})
	t.Run("verifier construction fails on bad RPC URL", func(t *testing.T) {
		config.ClearPMWPaymentStatusConfigForTest()
		// DBs connect (Docker up), but an unsupported RPC scheme fails the on-chain
		// initial-nonce binder at construction, so the service fails closed.
		badEnvConfig := config.EnvConfig{
			SourceDatabaseURL:              "postgres://username:password@localhost:5432/flare_xrp_indexer?sslmode=disable",
			CChainDatabaseURL:              "root:root@tcp(127.0.0.1:3306)/db?parseTime=true",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
			RPCURL:                         "ftp://bad-scheme",
			SourceID:                       "testXRP",
			AttestationType:                fdc2.PMWPaymentStatus,
		}
		service, err := NewPaymentService(badEnvConfig)
		require.ErrorContains(t, err, "cannot construct PMWPaymentStatus verifier")
		require.Nil(t, service)
	})
	config.ClearPMWPaymentStatusConfigForTest()
}
