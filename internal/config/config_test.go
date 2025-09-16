package config

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/stretchr/testify/require"
)

func TestEncodeAttestationOrSourceName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid short string",
			input:       "TEE",
			expectError: false,
		},
		{
			name:        "valid 32 byte string",
			input:       "12345678901234567890123456789012",
			expectError: false,
		},
		{
			name:        "string starting with 0x",
			input:       "0xABC",
			expectError: true,
		},
		{
			name:        "string too long",
			input:       "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeAttestationOrSourceName(tt.input)

			if (err != nil) != tt.expectError {
				t.Fatalf("EncodeAttestationOrSourceName(%q) error = %v, wantErr %v", tt.input, err, tt.expectError)
			}

			if !tt.expectError {
				require.Len(t, got, 32)
			}
		})
	}
}
func TestCheckMissingFields(t *testing.T) {
	fields := []string{
		EnvRPCURL,
		EnvRelayContractAddress,
		EnvTeeMachineRegistryContractAddress,
		EnvDatabaseURL,
		EnvCChainDatabaseURL,
	}
	t.Run("no missing fields", func(t *testing.T) {
		cfg := EnvConfig{
			RPCURL:                            "rpc",
			RelayContractAddress:              "relay",
			TeeMachineRegistryContractAddress: "tee",
			CChainDatabaseURL:                 "cchain",
			DatabaseURL:                       "dbURL",
		}
		err := CheckMissingFields(cfg, fields)
		require.NoError(t, err)
	})
	t.Run("some missing fields", func(t *testing.T) {
		cfg := EnvConfig{
			RPCURL:               "rpc",
			RelayContractAddress: "",
		}
		err := CheckMissingFields(cfg, fields)
		require.Error(t, err)
		require.Contains(t, err.Error(), EnvRelayContractAddress)
		require.Contains(t, err.Error(), EnvCChainDatabaseURL)
		require.Contains(t, err.Error(), EnvTeeMachineRegistryContractAddress)
	})
	t.Run("all missing fields", func(t *testing.T) {
		cfg := EnvConfig{}
		err := CheckMissingFields(cfg, fields)
		for _, f := range fields {
			require.Contains(t, err.Error(), f)
		}
	})
}

func TestLoadEncodedAndABI(t *testing.T) {
	type args struct {
		envConfig EnvConfig
	}

	tests := []struct {
		name           string
		input          args
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid availability check",
			input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: connector.AvailabilityCheck,
				},
			},
			expectError: false,
		},
		{
			name: "valid pmw payment status",
			input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: connector.PMWPaymentStatus,
				},
			},
			expectError: false,
		},
		{
			name: "valid pmw multisig account configured",
			input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: connector.PMWMultisigAccountConfigured,
				},
			},
			expectError: false,
		},
		{
			name: "invalid attestation type",
			input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: "UnknownType",
				},
			},
			expectError:    true,
			expectedErrMsg: "no ABI struct names defined for attestation type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadEncodedAndABI(tt.input.envConfig)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got.SourceIDPair.SourceIDEncoded)
				require.NotNil(t, got.AttestationTypePair.AttestationTypeEncoded)
				require.NotNil(t, got.ABIPair.Request)
				require.NotNil(t, got.ABIPair.Response)
			}
		})
	}
}

func TestGetABIArguments(t *testing.T) {
	origABI := connector.ConnectorMetaData.ABI
	defer func() { connector.ConnectorMetaData.ABI = origABI }()

	connector.ConnectorMetaData.ABI = `[
		{
			"constant": false,
			"inputs": [{"name": "arg1","type": "uint256"}],
			"name": "TestMethod",
			"outputs": [],
			"payable": false,
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`

	t.Run("valid struct", func(t *testing.T) {
		arg, err := getABIArguments("TestMethod")
		require.NoError(t, err)
		require.Equal(t, "uint256", arg.Type.String())
	})
	t.Run("method not found", func(t *testing.T) {
		_, err := getABIArguments("MissingMethod")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid method definition")
	})
	t.Run("invalid ABI", func(t *testing.T) {
		connector.ConnectorMetaData.ABI = "not json"
		_, err := getABIArguments("TestMethod")
		require.Contains(t, err.Error(), "failed to parse ABI")
	})
}
