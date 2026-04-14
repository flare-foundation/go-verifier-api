package config_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPMWFeeProofConfigError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := config.BuildPMWFeeProofConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL, TEE_INSTRUCTIONS_CONTRACT_ADDRESS")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1",
		}
		cfg, err := config.BuildPMWFeeProofConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}

func TestBuildPMWFeeProofConfigSuccess(t *testing.T) {
	envConfig := config.EnvConfig{
		SourceID:                       config.SourceTestXRP,
		AttestationType:                connector.PMWFeeProof,
		SourceDatabaseURL:              "postgres://localhost/test",
		CChainDatabaseURL:              "root:root@tcp(localhost)/db",
		TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1",
	}
	cfg, err := config.BuildPMWFeeProofConfig(envConfig)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "postgres://localhost/test", cfg.SourceDatabaseURL)
	require.Equal(t, "root:root@tcp(localhost)/db", cfg.CchainDatabaseURL)
	require.NotEqual(t, cfg.TeeInstructionsContractAddress, [20]byte{}, "address must not be zero")
	require.NotNil(t, cfg.ParsedTeeInstructionsABI)
}
