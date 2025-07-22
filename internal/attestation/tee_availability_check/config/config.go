package teeavailabilitycheckconfig

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

type TeeAvailabilityCheckConfig struct {
	SourceID                   string
	RelayContractAddress       string
	TeeRegistryContractAddress string
	RPCURL                     string
	GoogleRootCertificate      *x509.Certificate
}

func LoadTeeAvailabilityCheckConfig() (*TeeAvailabilityCheckConfig, error) {
	sourceID := os.Getenv("SOURCE_ID")
	if sourceID == "" {
		return nil, fmt.Errorf("SOURCE_ID not set in .env")
	}
	if len(sourceID) > 32 {
		return nil, fmt.Errorf("SOURCE_ID longer than 32 bytes")
	}
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
	rootCertBytes, err := os.ReadFile("../google_confidential_space_root.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to read Google root certificate: %v", err)
	}
	cert, err := decodeAndParsePEMCertificate(string(rootCertBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Google root certificate: %v", err)
	}

	return &TeeAvailabilityCheckConfig{
		SourceID:                   sourceID,
		RelayContractAddress:       relayContractAddress,
		TeeRegistryContractAddress: teeRegistryContractAddress,
		RPCURL:                     rpcURL,
		GoogleRootCertificate:      cert,
	}, nil
}

// decodeAndParsePEMCertificate decodes the given PEM certificate string and parses it into an x509 certificate.
func decodeAndParsePEMCertificate(certificate string) (*x509.Certificate, error) {
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
