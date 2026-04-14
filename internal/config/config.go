package config

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

const (
	EnvRPCURL                            = "RPC_URL"
	EnvRelayContractAddress              = "RELAY_CONTRACT_ADDRESS"
	EnvTeeMachineRegistryContractAddress = "TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS"
	EnvTeeInstructionsContractAddress    = "TEE_INSTRUCTIONS_CONTRACT_ADDRESS"
	EnvSourceDatabaseURL                 = "SOURCE_DATABASE_URL"
	EnvCChainDatabaseURL                 = "CCHAIN_DATABASE_URL"
	EnvPort                              = "PORT"
	EnvAPIKeys                           = "API_KEYS"
	EnvAttestationType                   = "VERIFIER_TYPE"
	EnvSourceID                          = "SOURCE_ID"
	EnvAllowTeeDebug                     = "ALLOW_TEE_DEBUG"               // Needed only for test deployment. Not mandatory to set. Defaults to false.
	EnvDisableAttestationCheckE2E        = "DISABLE_ATTESTATION_CHECK_E2E" // Needed only for e2e test. Not mandatory to set. Defaults to false.
	EnvAllowPrivateNetworks              = "ALLOW_PRIVATE_NETWORKS"        // Test/E2E only. Allows private/loopback IPs while still blocking dangerous IPs. Defaults to false.
	EnvMaxPolledTees                     = "MAX_POLLED_TEES"               // Maximum total TEEs to poll. Extension 0 TEEs are always polled regardless of this value. 0 = extension 0 only (default). >0 = also poll extra TEEs from other extensions, up to this total.
)

type EnvConfig struct {
	RPCURL                            string
	RelayContractAddress              string
	TeeMachineRegistryContractAddress string
	TeeInstructionsContractAddress    string
	SourceDatabaseURL                 string
	CChainDatabaseURL                 string
	AllowTeeDebug                     string
	DisableAttestationCheckE2E        string
	AllowPrivateNetworks              string
	MaxPolledTees                     string
	Port                              string
	APIKeys                           []string
	AttestationType                   connector.AttestationType
	SourceID                          SourceName
}

type SourceName string

const (
	SourceTEE     SourceName = "TEE"
	SourceXRP     SourceName = "XRP"
	SourceTestXRP SourceName = "testXRP"
)

type SourceIDEncodedPair struct {
	SourceID        SourceName
	SourceIDEncoded common.Hash
}

type AttestationTypeEncodedPair struct {
	AttestationType        connector.AttestationType
	AttestationTypeEncoded common.Hash
}

type ABIArgPair struct {
	Request  abi.Argument
	Response abi.Argument
}

type TeeAvailabilityCheckConfig struct {
	EncodedAndABI
	RelayContractAddress              common.Address
	TeeMachineRegistryContractAddress common.Address
	AllowTeeDebug                     bool
	DisableAttestationCheckE2E        bool
	AllowPrivateNetworks              bool
	MaxPolledTees                     int
	RPCURL                            string
	GoogleRootCertificate             *x509.Certificate
}

type PMWPaymentStatusConfig struct {
	EncodedAndABI
	SourceDatabaseURL              string
	CchainDatabaseURL              string
	TeeInstructionsContractAddress common.Address
	ParsedTeeInstructionsABI       abi.ABI
}

type PMWFeeProofConfig struct {
	EncodedAndABI
	SourceDatabaseURL              string
	CchainDatabaseURL              string
	TeeInstructionsContractAddress common.Address
	ParsedTeeInstructionsABI       abi.ABI
}

type PMWMultisigAccountConfig struct {
	EncodedAndABI
	RPCURL string
}

type EncodedAndABI struct {
	SourceIDPair        SourceIDEncodedPair
	AttestationTypePair AttestationTypeEncodedPair
	ABIPair             ABIArgPair
}

func EncodeAttestationOrSourceName(attestationTypeOrSourceName string) (common.Hash, error) {
	if len(attestationTypeOrSourceName) >= 2 && (attestationTypeOrSourceName[:2] == "0x" || attestationTypeOrSourceName[:2] == "0X") {
		return common.Hash{}, fmt.Errorf("attestation type or source id name must not start with '0x'. Provided: %s", attestationTypeOrSourceName)
	}
	return convert.StringToCommonHash(attestationTypeOrSourceName)
}

var abiStructNames = map[connector.AttestationType]struct {
	Request  string
	Response string
}{
	connector.AvailabilityCheck: {
		Request:  "availabilityCheckRequestBodyStruct",
		Response: "availabilityCheckResponseBodyStruct",
	},
	connector.PMWMultisigAccountConfigured: {
		Request:  "pmwMultisigAccountConfiguredRequestBodyStruct",
		Response: "pmwMultisigAccountConfiguredResponseBodyStruct",
	},
	connector.PMWPaymentStatus: {
		Request:  "pmwPaymentStatusRequestBodyStruct",
		Response: "pmwPaymentStatusResponseBodyStruct",
	},
	connector.PMWFeeProof: {
		Request:  "pmwFeeProofRequestBodyStruct",
		Response: "pmwFeeProofResponseBodyStruct",
	},
}

func LoadEncodedAndABI(envConfig EnvConfig) (EncodedAndABI, error) {
	names, ok := abiStructNames[envConfig.AttestationType]
	if !ok {
		return EncodedAndABI{}, fmt.Errorf("no ABI struct names defined for attestation type %s", envConfig.AttestationType)
	}
	sourceIDEnc, err := EncodeAttestationOrSourceName(string(envConfig.SourceID))
	if err != nil {
		return EncodedAndABI{}, err
	}
	attestationTypeEnc, err := EncodeAttestationOrSourceName(string(envConfig.AttestationType))
	if err != nil {
		return EncodedAndABI{}, err
	}
	requestABI, err := getABIArguments(names.Request)
	if err != nil {
		return EncodedAndABI{}, err
	}
	responseABI, err := getABIArguments(names.Response)
	if err != nil {
		return EncodedAndABI{}, err
	}
	return EncodedAndABI{
		SourceIDPair:        SourceIDEncodedPair{SourceID: envConfig.SourceID, SourceIDEncoded: sourceIDEnc},
		AttestationTypePair: AttestationTypeEncodedPair{AttestationType: envConfig.AttestationType, AttestationTypeEncoded: attestationTypeEnc},
		ABIPair:             ABIArgPair{Request: requestABI, Response: responseABI},
	}, nil
}

func CheckMissingFields(cfg EnvConfig, fields []string) error {
	missing := []string{}
	for _, field := range fields {
		switch field {
		case EnvRPCURL:
			if cfg.RPCURL == "" {
				missing = append(missing, field)
			}
		case EnvRelayContractAddress:
			if cfg.RelayContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvTeeMachineRegistryContractAddress:
			if cfg.TeeMachineRegistryContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvTeeInstructionsContractAddress:
			if cfg.TeeInstructionsContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvSourceDatabaseURL:
			if cfg.SourceDatabaseURL == "" {
				missing = append(missing, field)
			}
		case EnvCChainDatabaseURL:
			if cfg.CChainDatabaseURL == "" {
				missing = append(missing, field)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getABIArguments(structNeeded string) (abi.Argument, error) {
	parsedABI, err := abi.JSON(strings.NewReader(connector.ConnectorMetaData.ABI))
	if err != nil {
		return abi.Argument{}, fmt.Errorf("failed to parse ABI: %w", err)
	}

	method, ok := parsedABI.Methods[structNeeded]
	if !ok || len(method.Inputs) != 1 {
		return abi.Argument{}, fmt.Errorf("invalid method definition for %s", structNeeded)
	}

	return method.Inputs[0], nil
}
