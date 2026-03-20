package config_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadPMWFeeProofConfigError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := config.LoadPMWFeeProofConfig(envConfig)
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
		cfg, err := config.LoadPMWFeeProofConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}

func TestLoadPMWFeeProofConfigSuccess(t *testing.T) {
	envConfig := config.EnvConfig{
		SourceID:          config.SourceTestXRP,
		AttestationType:   connector.PMWFeeProof,
		SourceDatabaseURL: "postgres://localhost/test",
		CChainDatabaseURL: "root:root@tcp(localhost)/db",
	}
	cfg, err := config.LoadPMWFeeProofConfig(envConfig)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "postgres://localhost/test", cfg.SourceDatabaseURL)
	require.Equal(t, "root:root@tcp(localhost)/db", cfg.CchainDatabaseURL)
	require.NotNil(t, cfg.ParsedTeeInstructionsABI)
}
