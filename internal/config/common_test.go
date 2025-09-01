package config_test

import (
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEncodeAttestationOrSourceName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid short string",
			input:   "TEE",
			wantErr: false,
		},
		{
			name:    "valid 32 byte string",
			input:   "12345678901234567890123456789012",
			wantErr: false,
		},
		{
			name:    "string starting with 0x",
			input:   "0xABC",
			wantErr: true,
		},
		{
			name:    "string too long",
			input:   "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.EncodeAttestationOrSourceName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EncodeAttestationOrSourceName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil {
				if len(got) != 2+utils.Bytes32Size*2 {
					t.Errorf("encoded length = %d, want %d", len(got), 2+utils.Bytes32Size*2)
				}
			}
		})
	}
}

func TestCheckMissingFields(t *testing.T) {
	fields := []string{
		config.EnvRPCURL,
		config.EnvRelayContractAddress,
		config.EnvTeeMachineRegistryContractAddress,
		config.EnvTeeWalletManagerContractAddress,
		config.EnvTeeWalletProjectManagerContractAddress,
		config.EnvDatabaseURL,
		config.EnvCChainDatabaseURL,
	}
	t.Run("no missing fields", func(t *testing.T) {
		cfg := config.EnvConfig{
			RPCURL:                                 "rpc",
			RelayContractAddress:                   "relay",
			TeeMachineRegistryContractAddress:      "tee",
			CChainDatabaseURL:                      "cchain",
			TeeWalletManagerContractAddress:        "walletManager",
			TeeWalletProjectManagerContractAddress: "walletProjectManager",
			DatabaseURL:                            "dbUrl",
		}
		err := config.CheckMissingFields(cfg, fields)
		require.NoError(t, err)
	})
	t.Run("some missing fields", func(t *testing.T) {
		cfg := config.EnvConfig{
			RPCURL:               "rpc",
			RelayContractAddress: "",
		}
		err := config.CheckMissingFields(cfg, fields)
		require.Error(t, err)
		require.Contains(t, err.Error(), config.EnvRelayContractAddress)
		require.Contains(t, err.Error(), config.EnvCChainDatabaseURL)
		require.Contains(t, err.Error(), config.EnvTeeMachineRegistryContractAddress)
	})
	t.Run("all missing fields", func(t *testing.T) {
		cfg := config.EnvConfig{}
		err := config.CheckMissingFields(cfg, fields)
		for _, f := range fields {
			require.Contains(t, err.Error(), f)
		}
	})
}
