package config_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/test_util"
	"github.com/stretchr/testify/require"
)

func TestEncodeAttestationOrSourceName(t *testing.T) {
	tests := []testutil.TestCase[string, any]{
		{
			Name:        "valid short string",
			Input:       "TEE",
			ExpectError: false,
		},
		{
			Name:        "valid 32 byte string",
			Input:       "12345678901234567890123456789012",
			ExpectError: false,
		},
		{
			Name:        "string starting with 0x",
			Input:       "0xABC",
			ExpectError: true,
		},
		{
			Name:        "string too long",
			Input:       "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz",
			ExpectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := config.EncodeAttestationOrSourceName(tt.Input)
			if (err != nil) != tt.ExpectError {
				t.Fatalf("EncodeAttestationOrSourceName(%q) error = %v, wantErr %v", tt.Input, err, tt.ExpectError)
			}
			require.Equal(t, 32, len(got))
		})
	}
}

func TestCheckMissingFields(t *testing.T) {
	fields := []string{
		config.EnvRPCURL,
		config.EnvRelayContractAddress,
		config.EnvTeeMachineRegistryContractAddress,
		config.EnvDatabaseURL,
		config.EnvCChainDatabaseURL,
	}
	t.Run("no missing fields", func(t *testing.T) {
		cfg := config.EnvConfig{
			RPCURL:                            "rpc",
			RelayContractAddress:              "relay",
			TeeMachineRegistryContractAddress: "tee",
			CChainDatabaseURL:                 "cchain",
			DatabaseURL:                       "dbUrl",
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

func TestLoadEncodedAndAbi(t *testing.T) {
	type args struct {
		envConfig config.EnvConfig
	}
	tests := []testutil.TestCase[args, any]{
		{
			Input: args{
				envConfig: config.EnvConfig{
					SourceID:        config.SourceTEE,
					AttestationType: connector.AvailabilityCheck,
				},
			},
			ExpectError: false,
		},
		{
			Input: args{
				envConfig: config.EnvConfig{
					SourceID:        config.SourceTEE,
					AttestationType: connector.PMWPaymentStatus,
				},
			},
			ExpectError: false,
		},
		{
			Input: args{
				envConfig: config.EnvConfig{
					SourceID:        config.SourceTEE,
					AttestationType: connector.PMWMultisigAccountConfigured,
				},
			},
			ExpectError: false,
		},
		{
			Input: args{
				envConfig: config.EnvConfig{
					SourceID:        config.SourceTEE,
					AttestationType: "UnknownType",
				},
			},
			ExpectError:    true,
			ExpectedErrMsg: "no ABI struct names defined for attestation type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := config.LoadEncodedAndAbi(tt.Input.envConfig)
			if tt.ExpectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.ExpectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got.SourceIdPair.SourceIdEncoded)
				require.NotNil(t, got.AttestationTypePair.AttestationTypeEncoded)
				require.NotNil(t, got.AbiPair.Request)
				require.NotNil(t, got.AbiPair.Response)
			}
		})
	}
}
