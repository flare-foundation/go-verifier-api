package config_test

import (
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPMWMultisigUtxoConfiguredConfigError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceBTC,
			AttestationType: "UnknownType",
		}
		cfg, err := config.BuildPMWMultisigUtxoConfiguredConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: SOURCE_RPC_URL")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceBTC,
			AttestationType: "UnknownType",
			SourceRPCURL:    "URL",
		}
		cfg, err := config.BuildPMWMultisigUtxoConfiguredConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}
