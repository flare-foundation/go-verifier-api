package teeavailabilitycheckconfig

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"os"
	"sync"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	teeAvailabilityCheckConfig     *config.TeeAvailabilityCheckConfig
	teeAvailabilityCheckConfigOnce sync.Once
	teeAvailabilityCheckConfigErr  error
)

func GetTeeAvailabilityCheckConfig(sourceId config.SourceName, attestationType connector.AttestationType) (*config.TeeAvailabilityCheckConfig, error) {
	teeAvailabilityCheckConfigOnce.Do(func() {
		teeAvailabilityCheckConfig, teeAvailabilityCheckConfigErr = LoadTeeAvailabilityCheckConfig(sourceId, attestationType)
	})
	return teeAvailabilityCheckConfig, teeAvailabilityCheckConfigErr
}

func LoadTeeAvailabilityCheckConfig(sourceId config.SourceName, attestationType connector.AttestationType) (*config.TeeAvailabilityCheckConfig, error) {
	relayContractAddress := os.Getenv("RELAY_CONTRACT_ADDRESS")
	if relayContractAddress == "" {
		return nil, fmt.Errorf("RELAY_CONTRACT_ADDRESS not set in .env")
	}
	teeRegistryContractAddress := os.Getenv("TEE_REGISTRY_CONTRACT_ADDRESS")
	if teeRegistryContractAddress == "" {
		return nil, fmt.Errorf("TEE_REGISTRY_CONTRACT_ADDRESS not set in .env")
	}
	rpcURL := os.Getenv("RPC_URL")
	if teeRegistryContractAddress == "" {
		return nil, fmt.Errorf("RPC_URL not set in .env")
	}
	googleRootCert, err := LoadGoogleRootCert()
	if err != nil {
		return nil, err
	}
	sourceIdEnc, err := config.EncodeAttestationOrSourceName(string(sourceId))
	if err != nil {
		return nil, err
	}
	attestationTypeEnc, err := config.EncodeAttestationOrSourceName(string(attestationType))
	if err != nil {
		return nil, err
	}
	return &config.TeeAvailabilityCheckConfig{
		SourcePair:                 config.SourceIdEncodedPair{SourceId: sourceId, SourceIdEncoded: sourceIdEnc},
		RelayContractAddress:       relayContractAddress,
		TeeRegistryContractAddress: teeRegistryContractAddress,
		RPCURL:                     rpcURL,
		GoogleRootCertificate:      googleRootCert,
		AttestationTypePair:        config.AttestationTypeEncodedPair{AttestationType: attestationType, AttestationTypeEncoded: attestationTypeEnc},
	}, nil
}

//go:embed assets/google_confidential_space_root.crt
var rootCertBytes []byte

func LoadGoogleRootCert() (*x509.Certificate, error) {
	cert, err := DecodeAndParsePEMCertificate(string(rootCertBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse root certificate: %w", err)
	}
	return cert, nil
}

// DecodeAndParsePEMCertificate decodes the given PEM certificate string and parses it into an x509 certificate.
func DecodeAndParsePEMCertificate(certificate string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		return nil, fmt.Errorf("cannot decode certificate: invalid PEM format")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse certificate: %v", err)
	}
	return cert, nil
}
