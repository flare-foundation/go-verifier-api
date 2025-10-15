package config_test

import (
	"testing"

	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/config"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadPMWMultisigAccountConfiguredConfigError(t *testing.T) {
	t.Run("missing variable", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := pmwmultisigaccountconfig.LoadPMWMultisigAccountConfiguredConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: RPC_URL")
	})
	t.Run("missing variable", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
			RPCURL:          "URL",
		}
		cfg, err := pmwmultisigaccountconfig.LoadPMWMultisigAccountConfiguredConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}
