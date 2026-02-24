package config

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"strconv"
	"sync"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

var (
	teeAvailabilityCheckConfig     *TeeAvailabilityCheckConfig
	teeAvailabilityCheckConfigOnce sync.Once
	errTeeAvailabilityCheckConfig  error
)

func GetTeeAvailabilityCheckConfig(envConfig EnvConfig) (*TeeAvailabilityCheckConfig, error) {
	teeAvailabilityCheckConfigOnce.Do(func() {
		teeAvailabilityCheckConfig, errTeeAvailabilityCheckConfig = LoadTeeAvailabilityCheckConfig(envConfig)
	})
	return teeAvailabilityCheckConfig, errTeeAvailabilityCheckConfig
}

func LoadTeeAvailabilityCheckConfig(envConfig EnvConfig) (*TeeAvailabilityCheckConfig, error) {
	err := CheckMissingFields(envConfig, []string{
		EnvRelayContractAddress,
		EnvTeeMachineRegistryContractAddress,
		EnvRPCURL,
	})
	if err != nil {
		return nil, err
	}
	googleRootCert, err := LoadGoogleRootCert()
	if err != nil {
		return nil, err
	}
	commonConfig, err := LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	allowTeeDebug := getBoolOrSetFalse(EnvAllowTeeDebug, envConfig.AllowTeeDebug)
	disableAttestationCheckE2E := getBoolOrSetFalse(EnvDisableAttestationCheckE2E, envConfig.DisableAttestationCheckE2E)
	disableURLValidation := getBoolOrSetFalse(EnvDisableURLValidation, envConfig.DisableURLValidation)
	if allowTeeDebug {
		logger.Warnf("%s is enabled. This flag is meant for TEE debug mode or testing only and should NOT be used in production.", EnvAllowTeeDebug)
	}
	if disableAttestationCheckE2E {
		logger.Warnf("%s is enabled. This flag is meant for E2E tests only and should NOT be used in production.", EnvDisableAttestationCheckE2E)
	}
	if disableURLValidation {
		logger.Warnf("%s is enabled. This flag is meant for testing only and should NOT be used in production.", EnvDisableURLValidation)
	}
	return &TeeAvailabilityCheckConfig{
		EncodedAndABI:                     commonConfig,
		RelayContractAddress:              envConfig.RelayContractAddress,
		TeeMachineRegistryContractAddress: envConfig.TeeMachineRegistryContractAddress,
		AllowTeeDebug:                     allowTeeDebug,
		DisableAttestationCheckE2E:        disableAttestationCheckE2E,
		DisableURLValidation:              disableURLValidation,
		RPCURL:                            envConfig.RPCURL,
		GoogleRootCertificate:             googleRootCert,
	}, nil
}

func getBoolOrSetFalse(key, val string) bool {
	if val == "" {
		logger.Infof("%s not set, defaulting to false", key)
		return false
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		logger.Warnf("%s has invalid value %q, defaulting to false", key, val)
		return false
	}
	return b
}

//go:embed assets/google_confidential_space_root_20340116.crt
var rootCertBytes []byte

func LoadGoogleRootCert() (*x509.Certificate, error) {
	return loadGoogleRootCertFromBytes(rootCertBytes)
}

func loadGoogleRootCertFromBytes(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("cannot decode embedded Google root certificate: invalid PEM format")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse embedded Google root certificate: %w", err)
	}
	return cert, nil
}

// ClearTeeAvailabilityCheckConfigForTest is a test utility function that resets the tee availability check config.
func ClearTeeAvailabilityCheckConfigForTest() {
	teeAvailabilityCheckConfig = nil
	errTeeAvailabilityCheckConfig = nil
	teeAvailabilityCheckConfigOnce = sync.Once{}
}
