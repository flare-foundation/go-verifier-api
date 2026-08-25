package config_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
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
		require.ErrorContains(t, err, "missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL, FLARE_TEE_MANAGER_CONTRACT_ADDRESS, TEE_PAYMENTS_CONTRACT_ADDRESS, FLARE_RPC_URL")
	})
	t.Run("missing TEE_PAYMENTS_CONTRACT_ADDRESS", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTestXRP,
			AttestationType:                fdc2.PMWPaymentStatus,
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			FlareRPCURL:                    "http://127.0.0.1:8545",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: TEE_PAYMENTS_CONTRACT_ADDRESS")
	})
	t.Run("missing FLARE_RPC_URL", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTestXRP,
			AttestationType:                fdc2.PMWPaymentStatus,
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: FLARE_RPC_URL")
	})
	t.Run("invalid FLARE_TEE_MANAGER_CONTRACT_ADDRESS hex", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			FlareTeeManagerContractAddress: "not-hex",
			TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
			FlareRPCURL:                    "http://127.0.0.1:8545",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "FLARE_TEE_MANAGER_CONTRACT_ADDRESS is not a valid hex address")
	})
	t.Run("invalid TEE_PAYMENTS_CONTRACT_ADDRESS hex", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			TeePaymentsContractAddress:     "not-hex",
			FlareRPCURL:                    "http://127.0.0.1:8545",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "TEE_PAYMENTS_CONTRACT_ADDRESS is not a valid hex address")
	})
	t.Run("zero FLARE_TEE_MANAGER_CONTRACT_ADDRESS rejected", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			FlareTeeManagerContractAddress: "0x0000000000000000000000000000000000000000",
			TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
			FlareRPCURL:                    "http://127.0.0.1:8545",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "FLARE_TEE_MANAGER_CONTRACT_ADDRESS must not be the zero address")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:                       config.SourceTEE,
			AttestationType:                "UnknownType",
			SourceDatabaseURL:              "URL",
			CChainDatabaseURL:              "URL",
			FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
			TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
			FlareRPCURL:                    "http://127.0.0.1:8545",
		}
		cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}

// btcEnv returns a fully-populated BTC (node-mode) PMWPaymentStatus env config;
// individual subtests blank or corrupt one field to exercise a validation branch.
func btcEnv() config.EnvConfig {
	return config.EnvConfig{
		SourceID:                   config.SourceTestBTC,
		AttestationType:            fdc2.PMWPaymentStatus,
		CChainDatabaseURL:          "root:root@tcp(localhost)/db",
		TeePaymentsContractAddress: "0x00000000000000000000000000000000000000C2",
		FlareRPCURL:                "http://127.0.0.1:8545",
		ChannelAddress:             "0x00000000000000000000000000000000000000C1",
		SourceRPCURL:               "http://127.0.0.1:8332",
		BtcNetwork:                 "signet",
	}
}

// TestBtcNetworkParamsOverrides: an explicit BTC_NETWORK overrides the source
// default, mapping each supported name to its chain params.
func TestBtcNetworkParamsOverrides(t *testing.T) {
	for _, c := range []struct{ network, wantName string }{
		{"mainnet", "mainnet"},
		{"testnet3", "testnet3"},
		{"signet", "signet"},
		{"regtest", "regtest"},
	} {
		params, err := config.BtcNetworkParams(config.SourceTestBTC, c.network)
		require.NoError(t, err)
		require.Equal(t, c.wantName, params.Name)
	}
	_, err := config.BtcNetworkParams(config.SourceTestBTC, "nosuchnet")
	require.Error(t, err)
}

func TestBuildBtcPMWPaymentStatusConfigSuccess(t *testing.T) {
	cfg, err := config.BuildPMWPaymentStatusConfig(btcEnv())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, common.HexToAddress("0xC1"), cfg.ChannelAddress)
	require.Equal(t, common.HexToAddress("0xC2"), cfg.TeePaymentsContractAddress)
	require.Equal(t, "http://127.0.0.1:8332", cfg.SourceRPCURL)
	require.Equal(t, "signet", cfg.BtcNetwork)
	// The XRP-only source database is not part of the BTC node-mode config.
	require.Empty(t, cfg.SourceDatabaseURL)
}

func TestBuildBtcPMWPaymentStatusConfigError(t *testing.T) {
	t.Run("missing node and channel fields", func(t *testing.T) {
		cfg, err := config.BuildPMWPaymentStatusConfig(config.EnvConfig{SourceID: config.SourceBTC})
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "CHANNEL_ADDRESS")
		require.ErrorContains(t, err, "SOURCE_RPC_URL")
	})
	t.Run("invalid CHANNEL_ADDRESS hex", func(t *testing.T) {
		env := btcEnv()
		env.ChannelAddress = "not-hex"
		cfg, err := config.BuildPMWPaymentStatusConfig(env)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "CHANNEL_ADDRESS is not a valid hex address")
	})
	t.Run("unresolvable BTC network", func(t *testing.T) {
		env := btcEnv()
		env.BtcNetwork = "nosuchnet"
		cfg, err := config.BuildPMWPaymentStatusConfig(env)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "nosuchnet")
	})
	t.Run("invalid TEE_PAYMENTS_CONTRACT_ADDRESS hex", func(t *testing.T) {
		env := btcEnv()
		env.TeePaymentsContractAddress = "not-hex"
		cfg, err := config.BuildPMWPaymentStatusConfig(env)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "TEE_PAYMENTS_CONTRACT_ADDRESS is not a valid hex address")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		env := btcEnv()
		env.AttestationType = "UnknownType"
		cfg, err := config.BuildPMWPaymentStatusConfig(env)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "UnknownType")
	})
}

func TestBuildPMWPaymentStatusConfigSuccess(t *testing.T) {
	config.ClearPMWPaymentStatusConfigForTest()
	envConfig := config.EnvConfig{
		SourceID:                       config.SourceTestXRP,
		AttestationType:                fdc2.PMWPaymentStatus,
		SourceDatabaseURL:              "postgres://localhost/test",
		CChainDatabaseURL:              "root:root@tcp(localhost)/db",
		FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
		TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
		FlareRPCURL:                    "http://127.0.0.1:8545",
	}
	cfg, err := config.BuildPMWPaymentStatusConfig(envConfig)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, common.HexToAddress("0xC1"), cfg.FlareTeeManagerContractAddress)
	require.Equal(t, common.HexToAddress("0xC2"), cfg.TeePaymentsContractAddress)
	require.Equal(t, "http://127.0.0.1:8545", cfg.FlareRPCURL)
	require.NotNil(t, cfg.ParsedTeeInstructionsABI)
}
