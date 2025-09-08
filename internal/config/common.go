package config

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

const (
	EnvRPCURL                                 = "RPC_URL"
	EnvRelayContractAddress                   = "RELAY_CONTRACT_ADDRESS"
	EnvTeeMachineRegistryContractAddress      = "TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS"
	EnvTeeWalletManagerContractAddress        = "TEE_WALLET_MANAGER_CONTRACT_ADDRESS"
	EnvTeeWalletProjectManagerContractAddress = "TEE_WALLET_PROJECT_MANAGER_CONTRACT_ADDRESS"
	EnvDatabaseURL                            = "DATABASE_URL"
	EnvCChainDatabaseURL                      = "CCHAIN_DATABASE_URL"
	EnvEnv                                    = "ENV"
	EnvPort                                   = "PORT"
	EnvAPIKeys                                = "API_KEYS"
	EnvAttestationType                        = "VERIFIER_TYPE"
	EnvSourceID                               = "SOURCE_ID"
)

type EnvConfig struct {
	RPCURL                                 string
	RelayContractAddress                   string
	TeeMachineRegistryContractAddress      string
	TeeWalletManagerContractAddress        string
	TeeWalletProjectManagerContractAddress string
	DatabaseURL                            string
	CChainDatabaseURL                      string
	Env                                    string
	Port                                   string
	APIKeys                                []string
	AttestationType                        connector.AttestationType
	SourceID                               SourceName
}

type SourceName string

const (
	SourceTEE SourceName = "TEE"
	SourceXRP SourceName = "XRP"
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
	RelayContractAddress              string
	TeeMachineRegistryContractAddress string
	RPCURL                            string
	GoogleRootCertificate             *x509.Certificate
}

type PMWPaymentStatusConfig struct {
	EncodedAndABI
	DatabaseURL              string
	CchainDatabaseURL        string
	ParsedTeeInstructionsABI abi.ABI
	ParsedPaymentABI         abi.ABI
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
		return common.Hash{}, fmt.Errorf("attestation type or source id name must not start with '0x'. Provided: '%s'", attestationTypeOrSourceName)
	}
	bytes := []byte(attestationTypeOrSourceName)
	if len(bytes) > utils.Bytes32Size {
		return common.Hash{}, fmt.Errorf("attestation type or source id name '%s' is too long (%d bytes)", attestationTypeOrSourceName, len(bytes))
	}
	padded := make([]byte, utils.Bytes32Size)
	copy(padded, bytes)
	return common.BytesToHash(padded), nil
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
	requestABI, err := GetABIArguments(names.Request)
	if err != nil {
		return EncodedAndABI{}, err
	}
	responseABI, err := GetABIArguments(names.Response)
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
		case EnvTeeWalletManagerContractAddress:
			if cfg.TeeWalletManagerContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvTeeWalletProjectManagerContractAddress:
			if cfg.TeeWalletProjectManagerContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvDatabaseURL:
			if cfg.DatabaseURL == "" {
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
