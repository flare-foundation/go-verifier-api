package config

import (
	"crypto/x509"
	"encoding/hex"
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
)

type SourceName string

const (
	SourceTEE SourceName = "tee"
	SourceXRP SourceName = "xrp"
)

type SourceIdEncodedPair struct {
	SourceId        SourceName
	SourceIdEncoded string
}

type AttestationTypeEncodedPair struct {
	AttestationType        connector.AttestationType
	AttestationTypeEncoded string
}

type TeeAvailabilityCheckConfig struct {
	SourcePair                 SourceIdEncodedPair
	RelayContractAddress       string
	TeeRegistryContractAddress string
	RPCURL                     string
	GoogleRootCertificate      *x509.Certificate
	AttestationTypePair        AttestationTypeEncodedPair
}

type PMWPaymentStatusConfig struct {
	SourcePair          SourceIdEncodedPair
	DatabaseURL         string
	CchainDatabaseURL   string
	AttestationTypePair AttestationTypeEncodedPair
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
