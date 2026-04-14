package config_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPMWPaymentStatusConfigError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL, TEE_INSTRUCTIONS_CONTRACT_ADDRESS")
	})
	t.Run("invalid TEE_INSTRUCTIONS_CONTRACT_ADDRESS hex", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			TeeInstructionsContractAddress: "not-hex",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "TEE_INSTRUCTIONS_CONTRACT_ADDRESS is not a valid hex address")
	})
	t.Run("zero TEE_INSTRUCTIONS_CONTRACT_ADDRESS rejected", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			TeeInstructionsContractAddress: "0x0000000000000000000000000000000000000000",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "TEE_INSTRUCTIONS_CONTRACT_ADDRESS must not be the zero address")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}

func TestBuildPMWPaymentStatusConfigSuccess(t *testing.T) {
	config.ClearPMWPaymentStatusConfigForTest()
	envConfig := config.EnvConfig{
		SourceID:                       config.SourceTestXRP,
		AttestationType:                connector.PMWPaymentStatus,
		SourceDatabaseURL:              "postgres://localhost/test",
		CChainDatabaseURL:              "root:root@tcp(localhost)/db",
		TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1",
	}
	cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, common.HexToAddress("0xC1"), cfg.TeeInstructionsContractAddress)
	require.NotNil(t, cfg.ParsedTeeInstructionsABI)
}
