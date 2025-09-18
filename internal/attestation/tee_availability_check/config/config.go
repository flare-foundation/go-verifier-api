package teeavailabilitycheckconfig

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
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
	err := config.CheckMissingFields(envConfig, []string{config.EnvRelayContractAddress, config.EnvTeeMachineRegistryContractAddress, config.EnvRPCURL, config.EnvAllowTeeDebug, config.EnvDisableAttestationCheckE2E})
	if err != nil {
		return nil, err
	}
	if envConfig.DisableAttestationCheckE2E && config.CheckMissingFields(envConfig, []string{config.EnvE2ECodeHash, config.EnvE2EPlatform}) != nil {
		return nil, err
	}
	googleRootCert, err := LoadGoogleRootCert()
	if err != nil {
		return nil, err
	}
	commonConfig, err := config.LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}

	var e2eCodeHash, e2ePlatform common.Hash
	if envConfig.DisableAttestationCheckE2E {
		e2eCodeHash = common.HexToHash(envConfig.E2ECodeHash)
		e2ePlatform = common.HexToHash(envConfig.E2EPlatform)

		logger.Info(e2ePlatform, e2eCodeHash)
	}

	return &config.TeeAvailabilityCheckConfig{
		EncodedAndABI:                     commonConfig,
		RelayContractAddress:              envConfig.RelayContractAddress,
		TeeMachineRegistryContractAddress: envConfig.TeeMachineRegistryContractAddress,
		AllowTeeDebug:                     envConfig.AllowTeeDebug,
		DisableAttestationCheckE2E:        envConfig.DisableAttestationCheckE2E,
		RPCURL:                            envConfig.RPCURL,
		GoogleRootCertificate:             googleRootCert,
		E2ECodeHash:                       e2eCodeHash,
		E2EPlatform:                       e2ePlatform,
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

// ResetTeeAvailabilityCheckConfigForTest is a test utility function that resets the tee availability check config.
func ResetTeeAvailabilityCheckConfigForTest() {
	teeAvailabilityCheckConfig = nil
	errTeeAvailabilityCheckConfig = nil
	teeAvailabilityCheckConfigOnce = sync.Once{}
}
