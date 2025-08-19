package config

import (
	"crypto/x509"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

type SourceName string

const (
	SourceTEE     SourceName = "tee"
	SourceXRP     SourceName = "xrp"
	SourceTestXRP SourceName = "testxrp"
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
	SourcePair          SourceIdEncodedPair
	DatabaseURL         string
	CchainDatabaseURL   string
	AttestationTypePair AttestationTypeEncodedPair
	AbiPair             AbiArgPair
}

type PMWMultisigAccountConfig struct {
	SourcePair          SourceIdEncodedPair
	RPCURL              string
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
	return "0x" + hex.EncodeToString(padded), nil
}

type EncodedAndAbi struct {
	SourceIdPair        SourceIdEncodedPair
	AttestationTypePair AttestationTypeEncodedPair
	AbiPair             AbiArgPair
}

func LoadEncodedAndAbi(sourceId SourceName, attestationType connector.AttestationType, reqStructName, respStructName string) (EncodedAndAbi, error) {
	sourceIdEnc, err := EncodeAttestationOrSourceName(string(sourceId))
	if err != nil {
		return EncodedAndAbi{}, err
	}
	attestationTypeEnc, err := EncodeAttestationOrSourceName(string(attestationType))
	if err != nil {
		return EncodedAndAbi{}, err
	}
	requestAbi, err := GetAbiArguments(reqStructName)
	if err != nil {
		return EncodedAndAbi{}, err
	}
	responseAbi, err := GetAbiArguments(respStructName)
	if err != nil {
		return EncodedAndAbi{}, err
	}
	return EncodedAndAbi{
		SourceIdPair:        SourceIdEncodedPair{SourceId: sourceId, SourceIdEncoded: sourceIdEnc},
		AttestationTypePair: AttestationTypeEncodedPair{AttestationType: attestationType, AttestationTypeEncoded: attestationTypeEnc},
		AbiPair:             AbiArgPair{Request: requestAbi, Response: responseAbi},
	}, nil
}
