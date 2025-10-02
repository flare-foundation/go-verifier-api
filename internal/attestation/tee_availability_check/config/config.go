package teeavailabilitycheckconfig

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"strconv"
	"sync"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
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
	googleRootCert, err := LoadGoogleRootCert()
	if err != nil {
		return nil, err
	}
	commonConfig, err := config.LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	allowTeeDebug, err := getBoolOrError(config.EnvAllowTeeDebug, envConfig.AllowTeeDebug)
	if err != nil {
		return nil, err
	}
	disableAttestationCheckE2E, err := getBoolOrError(config.EnvDisableAttestationCheckE2E, envConfig.DisableAttestationCheckE2E)
	if err != nil {
		return nil, err
	}
	if allowTeeDebug {
		logger.Warnf("%s is enabled. This flag is meant for TEE debug mode or testing only and should NOT be used in production.", config.EnvAllowTeeDebug)
	}
	if disableAttestationCheckE2E {
		logger.Warnf("%s is enabled. This flag is meant for E2E tests only and should NOT be used in production.", config.EnvDisableAttestationCheckE2E)
	}
	return &config.TeeAvailabilityCheckConfig{
		EncodedAndABI:                     commonConfig,
		RelayContractAddress:              envConfig.RelayContractAddress,
		TeeMachineRegistryContractAddress: envConfig.TeeMachineRegistryContractAddress,
		AllowTeeDebug:                     allowTeeDebug,
		DisableAttestationCheckE2E:        disableAttestationCheckE2E,
		RPCURL:                            envConfig.RPCURL,
		GoogleRootCertificate:             googleRootCert,
	}, nil
}

func getBoolOrError(key, val string) (bool, error) {
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean, got %q", key, val)
	}
	return b, nil
}

//go:embed assets/google_confidential_space_root_20340116.crt
var rootCertBytes []byte

func LoadGoogleRootCert() (*x509.Certificate, error) {
	block, _ := pem.Decode(rootCertBytes)
	if block == nil {
		return nil, fmt.Errorf("cannot decode embedded Google root certificate: invalid PEM format")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse embedded Google root certificate: %w", err)
	}
	return cert, nil
}

// ResetTeeAvailabilityCheckConfigForTest is a test utility function that resets the tee availability check config.
func ResetTeeAvailabilityCheckConfigForTest() {
	teeAvailabilityCheckConfig = nil
	errTeeAvailabilityCheckConfig = nil
	teeAvailabilityCheckConfigOnce = sync.Once{}
}
