package utxomultisigservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	SourceRPCURL:    "http://user:pass@localhost:18332",
	SourceID:        "testBTC",
	AttestationType: fdc2.PMWMultisigUtxoConfigured,
}

func TestUtxoMultisigService(t *testing.T) {
	t.Run("should successfully create UtxoMultisigService", func(t *testing.T) {
		config.ClearPMWMultisigUtxoConfiguredConfigForTest()
		service, err := NewUtxoMultisigService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.Verifier())
		require.NotNil(t, service.Config())
	})

	t.Run("missing fields in env config", func(t *testing.T) {
		config.ClearPMWMultisigUtxoConfiguredConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceRPCURL:    "",
			SourceID:        "testBTC",
			AttestationType: fdc2.PMWMultisigUtxoConfigured,
		}
		service, err := NewUtxoMultisigService(badEnvConfig)
		require.ErrorContains(t, err, "cannot load PMWMultisigUtxoConfigured config: missing environment variables: SOURCE_RPC_URL")
		require.Nil(t, service)
	})

	t.Run("using unsupported source ID", func(t *testing.T) {
		config.ClearPMWMultisigUtxoConfiguredConfigForTest()
		badEnvConfig := config.EnvConfig{
			SourceRPCURL:    "http://user:pass@localhost:18332",
			SourceID:        "UNSUPPORTED_SOURCE",
			AttestationType: fdc2.PMWMultisigUtxoConfigured,
		}
		service, err := NewUtxoMultisigService(badEnvConfig)
		require.ErrorContains(t, err, "cannot initialize PMWMultisigUtxoConfigured verifier: no verifier for sourceID: UNSUPPORTED_SOURCE")
		require.Nil(t, service)
	})

	config.ClearPMWMultisigUtxoConfiguredConfigForTest()
}
