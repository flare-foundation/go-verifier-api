package config_test

import (
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadPMWPaymentStatusConfigError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := config.LoadPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:          config.SourceTEE,
			AttestationType:   "UnknownType",
			SourceDatabaseURL: "URL",
			CChainDatabaseURL: "URL",
		}
		cfg, err := config.LoadPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}
