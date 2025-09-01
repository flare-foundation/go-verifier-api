package config

import (
	"crypto/x509"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
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
	EnvApiKeys                                = "API_KEYS"
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
	ApiKeys                                []string
	AttestationType                        connector.AttestationType
	SourceID                               SourceName
}

type SourceName string

const (
	SourceTEE SourceName = "TEE"
	SourceXRP SourceName = "XRP"
)

type SourceIdEncodedPair struct {
	SourceId        SourceName
	SourceIdEncoded string
}

type AttestationTypeEncodedPair struct {
	AttestationType        connector.AttestationType
	AttestationTypeEncoded string
}

type AbiArgPair struct {
	Request  abi.Argument
	Response abi.Argument
}

type TeeAvailabilityCheckConfig struct {
	EncodedAndAbi
	RelayContractAddress       string
	TeeRegistryContractAddress string
	RPCURL                     string
	GoogleRootCertificate      *x509.Certificate
}

type PMWPaymentStatusConfig struct {
	EncodedAndAbi
	DatabaseURL              string
	CchainDatabaseURL        string
	ParsedTeeInstructionsABI abi.ABI
	ParsedPaymentABI         abi.ABI
}

type PMWMultisigAccountConfig struct {
	EncodedAndAbi
	RPCURL string
}

type EncodedAndAbi struct {
	SourceIdPair        SourceIdEncodedPair
	AttestationTypePair AttestationTypeEncodedPair
	AbiPair             AbiArgPair
}

func EncodeAttestationOrSourceName(attestationTypeOrSourceName string) (string, error) {
	if len(attestationTypeOrSourceName) >= 2 && (attestationTypeOrSourceName[:2] == "0x" || attestationTypeOrSourceName[:2] == "0X") {
		return "", fmt.Errorf("attestation type or source id name must not start with '0x'. Provided: '%s'", attestationTypeOrSourceName)
	}
	bytes := []byte(attestationTypeOrSourceName)
	if len(bytes) > utils.Bytes32Size {
		return "", fmt.Errorf("attestation type or source id name '%s' is too long (%d bytes)", attestationTypeOrSourceName, len(bytes))
	}
	padded := make([]byte, utils.Bytes32Size)
	copy(padded, bytes)
	return utils.BytesToHex0x(padded), nil
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

func LoadEncodedAndAbi(envConfig EnvConfig) (EncodedAndAbi, error) {
	names, ok := abiStructNames[envConfig.AttestationType]
	if !ok {
		return EncodedAndAbi{}, fmt.Errorf("no ABI struct names defined for attestation type %s", envConfig.AttestationType)
	}
	sourceIdEnc, err := EncodeAttestationOrSourceName(string(envConfig.SourceID))
	if err != nil {
		return EncodedAndAbi{}, err
	}
	attestationTypeEnc, err := EncodeAttestationOrSourceName(string(envConfig.AttestationType))
	if err != nil {
		return EncodedAndAbi{}, err
	}
	requestAbi, err := GetAbiArguments(names.Request)
	if err != nil {
		return EncodedAndAbi{}, err
	}
	responseAbi, err := GetAbiArguments(names.Response)
	if err != nil {
		return EncodedAndAbi{}, err
	}
	return EncodedAndAbi{
		SourceIdPair:        SourceIdEncodedPair{SourceId: envConfig.SourceID, SourceIdEncoded: sourceIdEnc},
		AttestationTypePair: AttestationTypeEncodedPair{AttestationType: envConfig.AttestationType, AttestationTypeEncoded: attestationTypeEnc},
		AbiPair:             AbiArgPair{Request: requestAbi, Response: responseAbi},
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
		return fmt.Errorf("missing environment variables: %v", missing)
	}
	return nil
}
