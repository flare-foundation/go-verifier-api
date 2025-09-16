package multisigservice

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/config"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

var envConfig = config.EnvConfig{
	RPCURL:          "https://s.altnet.rippletest.net:51234",
	SourceID:        "XRP",
	AttestationType: connector.PMWMultisigAccountConfigured,
}

func TestMultisigService(t *testing.T) {
	t.Run("Should successfully create MultisigService", func(t *testing.T) {
		service, err := NewMultisigService(envConfig)
		require.NoError(t, err)
		require.NotNil(t, service)
		require.NotNil(t, service.GetVerifier())
		require.NotNil(t, service.GetConfig())
	})

	t.Run("Missing fields in env config", func(t *testing.T) {
		pmwmultisigaccountconfig.ClearPMWMultisigAccountConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:          "",
			SourceID:        "XRP",
			AttestationType: connector.PMWMultisigAccountConfigured,
		}
		service, err := NewMultisigService(badEnvConfig)
		require.Error(t, err)
		require.Nil(t, service)
	})

	t.Run("Using unsupported source ID", func(t *testing.T) {
		pmwmultisigaccountconfig.ClearPMWMultisigAccountConfigForTest()
		badEnvConfig := config.EnvConfig{
			RPCURL:          "https://s.altnet.rippletest.net:51234",
			SourceID:        "UNSUPPORTED_SOURCE",
			AttestationType: connector.PMWMultisigAccountConfigured,
		}
		service, err := NewMultisigService(badEnvConfig)
		require.Error(t, err)
		require.Nil(t, service)
	})

	pmwmultisigaccountconfig.ClearPMWMultisigAccountConfigForTest()
}
