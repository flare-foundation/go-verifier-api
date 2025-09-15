package config

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestEncodeAttestationOrSourceName(t *testing.T) {
	tests := []testhelper.TestCase[string, any]{
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
			got, err := EncodeAttestationOrSourceName(tt.Input)
			if (err != nil) != tt.ExpectError {
				t.Fatalf("EncodeAttestationOrSourceName(%q) error = %v, wantErr %v", tt.Input, err, tt.ExpectError)
			}
			require.Len(t, got, 32)
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
	tests := []testhelper.TestCase[args, any]{
		{
			Input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: connector.AvailabilityCheck,
				},
			},
			ExpectError: false,
		},
		{
			Input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: connector.PMWPaymentStatus,
				},
			},
			ExpectError: false,
		},
		{
			Input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: connector.PMWMultisigAccountConfigured,
				},
			},
			ExpectError: false,
		},
		{
			Input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: "UnknownType",
				},
			},
			ExpectError:    true,
			ExpectedErrMsg: "no ABI struct names defined for attestation type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := LoadEncodedAndABI(tt.Input.envConfig)
			if tt.ExpectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.ExpectedErrMsg)
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
