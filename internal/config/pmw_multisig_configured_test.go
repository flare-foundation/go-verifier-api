package config_test

import (
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPMWMultisigAccountConfiguredConfigError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := config.BuildPMWMultisigAccountConfiguredConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: RPC_URL")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
			RPCURL:          "URL",
		}
		cfg, err := config.BuildPMWMultisigAccountConfiguredConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}
