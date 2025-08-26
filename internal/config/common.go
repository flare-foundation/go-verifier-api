package config

import (
	"crypto/x509"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/utils"
)

type EnvConfig struct {
	RPCURL                                 string
	XRPClientURL                           string
	RelayContractAddress                   string
	TeeRegistryContractAddress             string
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
	SourceTEE     SourceName = "TEE"
	SourceXRP     SourceName = "XRP"
	SourceTestXRP SourceName = "TESTXRP"
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
	SourcePair                 SourceIdEncodedPair
	RelayContractAddress       string
	TeeRegistryContractAddress string
	RPCURL                     string
	GoogleRootCertificate      *x509.Certificate
	AttestationTypePair        AttestationTypeEncodedPair
	AbiPair                    AbiArgPair
}

type PMWPaymentStatusConfig struct {
	SourcePair                     SourceIdEncodedPair
	DatabaseURL                    string
	CchainDatabaseURL              string
	RPCURL                         string
	TeeWalletManagerAddress        string
	TeeWalletProjectManagerAddress string
	AttestationTypePair            AttestationTypeEncodedPair
	AbiPair                        AbiArgPair
	ParsedTeeInstructionsABI       abi.ABI
	ParsedPaymentABI               abi.ABI
}

type PMWMultisigAccountConfig struct {
	SourcePair          SourceIdEncodedPair
	RPCURL              string
	AttestationTypePair AttestationTypeEncodedPair
	AbiPair             AbiArgPair
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
	if len(bytes) > 32 {
		return "", fmt.Errorf("attestation type or source id name '%s' is too long (%d bytes)", attestationTypeOrSourceName, len(bytes))
	}
	padded := make([]byte, 32)
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
