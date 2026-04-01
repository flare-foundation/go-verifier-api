package multisigservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:          "https://s.altnet.rippletest.net:51234",
	SourceID:        "testXRP",
	AttestationType: connector.PMWMultisigAccountConfigured,
}

func TestMultisigService(t *testing.T) {
	t.Run("should successfully create MultisigService", func(t *testing.T) {
		service, err := NewMultisigService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.Verifier())
		require.NotNil(t, service.Config())
	})

	t.Run("missing fields in env config", func(t *testing.T) {
		config.ClearPMWMultisigAccountConfiguredConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:          "",
			SourceID:        "testXRP",
			AttestationType: connector.PMWMultisigAccountConfigured,
		}
		service, err := NewMultisigService(badEnvConfig)
		require.ErrorContains(t, err, "cannot load PMWMultisigAccountConfigured config: missing environment variables: RPC_URL")
		require.Nil(t, service)
	})

	t.Run("using unsupported source ID", func(t *testing.T) {
		config.ClearPMWMultisigAccountConfiguredConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:          "https://s.altnet.rippletest.net:51234",
			SourceID:        "UNSUPPORTED_SOURCE",
			AttestationType: connector.PMWMultisigAccountConfigured,
		}
		service, err := NewMultisigService(badEnvConfig)
		require.ErrorContains(t, err, "cannot initialize PMWMultisigAccountConfigured verifier: no verifier for sourceID: UNSUPPORTED_SOURCE")
		require.Nil(t, service)
	})

	config.ClearPMWMultisigAccountConfiguredConfigForTest()
}
