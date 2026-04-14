package config

import (
	"maps"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/stretchr/testify/require"
)

func TestEncodeAttestationOrSourceName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{name: "valid short string", input: "TEE", expectError: false},
		{name: "valid 32 byte string", input: "12345678901234567890123456789012", expectError: false},
		{name: "string starting with 0x", input: "0xABC", expectError: true},
		{name: "string too long", input: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz", expectError: true},
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
		EnvSourceDatabaseURL,
		EnvCChainDatabaseURL,
	}
	t.Run("no missing fields", func(t *testing.T) {
		cfg := EnvConfig{
			RPCURL:                            "rpc",
			RelayContractAddress:              "relay",
			TeeMachineRegistryContractAddress: "tee",
			CChainDatabaseURL:                 "cchain",
			SourceDatabaseURL:                 "dbURL",
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
		require.ErrorContains(t, err, "missing environment variables: RELAY_CONTRACT_ADDRESS, TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS, SOURCE_DATABASE_URL, CCHAIN_DATABASE_URL")
		require.ErrorContains(t, err, EnvRelayContractAddress)
		require.ErrorContains(t, err, EnvCChainDatabaseURL)
		require.ErrorContains(t, err, EnvTeeMachineRegistryContractAddress)
	})
	t.Run("all missing fields", func(t *testing.T) {
		cfg := EnvConfig{}
		err := CheckMissingFields(cfg, fields)
		for _, f := range fields {
			require.ErrorContains(t, err, "missing environment variables:")
			require.ErrorContains(t, err, f)
		}
	})
	t.Run("TEE_INSTRUCTIONS_CONTRACT_ADDRESS missing", func(t *testing.T) {
		cfg := EnvConfig{}
		err := CheckMissingFields(cfg, []string{EnvTeeInstructionsContractAddress})
		require.ErrorContains(t, err, EnvTeeInstructionsContractAddress)
	})
	t.Run("TEE_INSTRUCTIONS_CONTRACT_ADDRESS set", func(t *testing.T) {
		cfg := EnvConfig{TeeInstructionsContractAddress: "0x00000000000000000000000000000000000000C1"}
		err := CheckMissingFields(cfg, []string{EnvTeeInstructionsContractAddress})
		require.NoError(t, err)
	})
}

func TestLoadEncodedAndABI(t *testing.T) {
	original := abiStructNames
	abiStructNames = maps.Clone(original)
	defer func() { abiStructNames = original }()

	testCases := map[connector.AttestationType]struct {
		Request, Response string
	}{
		"0xInvalidName":             {"req", "res"},
		connector.AvailabilityCheck: {"availabilityCheckRequestBodyStruct", "availabilityCheckResponseBodyStruct"},
		"InvalidRequestABI":         {"req", "availabilityCheckResponseBodyStruct"},
		"InvalidResponseABI":        {"availabilityCheckRequestBodyStruct", "res"},
	}
	maps.Copy(abiStructNames, testCases)

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
		{
			name: "invalid attestation type 2",
			input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: "0xInvalidName",
				},
			},
			expectError:    true,
			expectedErrMsg: "attestation type or source id name must not start with '0x'. Provided: 0xInvalidName",
		},
		{
			name: "invalid sourceID",
			input: args{
				envConfig: EnvConfig{
					SourceID:        "0xInvalidName",
					AttestationType: connector.PMWMultisigAccountConfigured,
				},
			},
			expectError:    true,
			expectedErrMsg: "attestation type or source id name must not start with '0x'. Provided: 0xInvalidName",
		},
		{
			name: "invalid request ABI",
			input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: "InvalidRequestABI",
				},
			},
			expectError:    true,
			expectedErrMsg: "invalid method definition for req",
		},
		{
			name: "invalid response ABI",
			input: args{
				envConfig: EnvConfig{
					SourceID:        SourceTEE,
					AttestationType: "InvalidResponseABI",
				},
			},
			expectError:    true,
			expectedErrMsg: "invalid method definition for res",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadEncodedAndABI(tt.input.envConfig)

			if tt.expectError {
				require.ErrorContains(t, err, tt.expectedErrMsg)
				require.Equal(t, EncodedAndABI{}, got)
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
		val, err := getABIArguments("MissingMethod")
		require.ErrorContains(t, err, "invalid method definition for MissingMethod")
		require.Equal(t, abi.Argument{}, val)
	})
	t.Run("invalid ABI", func(t *testing.T) {
		connector.ConnectorMetaData.ABI = "not json"
		val, err := getABIArguments("TestMethod")
		require.ErrorContains(t, err, "failed to parse ABI: invalid character")
		require.Equal(t, abi.Argument{}, val)
	})
}
