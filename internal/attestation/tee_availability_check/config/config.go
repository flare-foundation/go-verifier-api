package teeavailabilitycheckconfig

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"sync"

	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	teeAvailabilityCheckConfig     *config.TeeAvailabilityCheckConfig
	teeAvailabilityCheckConfigOnce sync.Once
	errTeeAvailabilityCheckConfig  error
)

func GetTeeAvailabilityCheckConfig(envConfig config.EnvConfig) (*config.TeeAvailabilityCheckConfig, error) {
	teeAvailabilityCheckConfigOnce.Do(func() {
		teeAvailabilityCheckConfig, errTeeAvailabilityCheckConfig = LoadTeeAvailabilityCheckConfig(envConfig)
	})
	return teeAvailabilityCheckConfig, errTeeAvailabilityCheckConfig
}

func LoadTeeAvailabilityCheckConfig(envConfig config.EnvConfig) (*config.TeeAvailabilityCheckConfig, error) {
	if envConfig.RelayContractAddress == "" {
		return nil, fmt.Errorf("RELAY_CONTRACT_ADDRESS not set in .env")
	}
	if envConfig.TeeRegistryContractAddress == "" {
		return nil, fmt.Errorf("TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS not set in .env")
	}
	if envConfig.RPCURL == "" {
		return nil, fmt.Errorf("RPC_URL not set in .env")
	}
	googleRootCert, err := LoadGoogleRootCert()
	if err != nil {
		return nil, err
	}
	commonConfig, err := config.LoadEncodedAndAbi(envConfig)
	if err != nil {
		return nil, err
	}
	return &config.TeeAvailabilityCheckConfig{
		SourcePair:                 commonConfig.SourceIdPair,
		RelayContractAddress:       envConfig.RelayContractAddress,
		TeeRegistryContractAddress: envConfig.TeeRegistryContractAddress,
		RPCURL:                     envConfig.RPCURL,
		GoogleRootCertificate:      googleRootCert,
		AttestationTypePair:        commonConfig.AttestationTypePair,
		AbiPair:                    commonConfig.AbiPair,
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
		return nil, fmt.Errorf("cannot parse certificate: %w", err)
	}
	return cert, nil
}
