package config

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
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

func LoadTeeAvailabilityCheckConfig(envConfig EnvConfig) (*TeeAvailabilityCheckConfig, error) {
	teeAvailabilityCheckConfigOnce.Do(func() {
		teeAvailabilityCheckConfig, errTeeAvailabilityCheckConfig = BuildTeeAvailabilityCheckConfig(envConfig)
	})
	return teeAvailabilityCheckConfig, errTeeAvailabilityCheckConfig
}

func BuildTeeAvailabilityCheckConfig(envConfig EnvConfig) (*TeeAvailabilityCheckConfig, error) {
	err := CheckMissingFields(envConfig, []string{
		EnvRelayContractAddress,
		EnvRPCURL,
	})
	if err != nil {
		return nil, err
	}
	relayAddr, err := parseContractAddress(envConfig.RelayContractAddress, EnvRelayContractAddress)
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
	allowTeeDebug, err := parseOptionalBool(EnvAllowTeeDebug, envConfig.AllowTeeDebug)
	if err != nil {
		return nil, err
	}
	disableAttestationCheckE2E, err := parseOptionalBool(EnvDisableAttestationCheckE2E, envConfig.DisableAttestationCheckE2E)
	if err != nil {
		return nil, err
	}
	allowPrivateNetworks, err := parseOptionalBool(EnvAllowPrivateNetworks, envConfig.AllowPrivateNetworks)
	if err != nil {
		return nil, err
	}
	if allowTeeDebug {
		logger.Warnf("%s is enabled. This flag is meant for TEE debug mode or testing only and should NOT be used in production.", EnvAllowTeeDebug)
	}
	if disableAttestationCheckE2E {
		logger.Warnf("%s is enabled. This flag is meant for E2E tests only and should NOT be used in production.", EnvDisableAttestationCheckE2E)
	}
	if allowPrivateNetworks {
		logger.Warnf("%s is enabled. This flag is meant for test/E2E environments only and should NOT be used in production. Private/loopback IPs will be allowed but dangerous IPs (link-local, metadata, multicast) are still blocked.", EnvAllowPrivateNetworks)
	}

	return &TeeAvailabilityCheckConfig{
		EncodedAndABI:              commonConfig,
		RelayContractAddress:       relayAddr,
		AllowTeeDebug:              allowTeeDebug,
		DisableAttestationCheckE2E: disableAttestationCheckE2E,
		AllowPrivateNetworks:       allowPrivateNetworks,
		RPCURL:                     envConfig.RPCURL,
		GoogleRootCertificate:      googleRootCert,
	}, nil
}

// parseOptionalBool parses an optional boolean env flag. An unset (empty) value
// defaults to false, but a non-empty value that is not a valid bool is a
// misconfiguration and fails the boot — rather than being silently swallowed to
// false, which would discard operator intent (e.g. a typo'd ALLOW_TEE_DEBUG).
func parseOptionalBool(key, val string) (bool, error) {
	if val == "" {
		logger.Infof("%s not set, defaulting to false", key)
		return false, nil
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf("%s has invalid bool value %q: %w", key, val, err)
	}
	return b, nil
}

//go:embed assets/google_confidential_space_root_20340116.crt
var rootCertBytes []byte

func LoadGoogleRootCert() (*x509.Certificate, error) {
	return loadGoogleRootCertFromBytes(rootCertBytes)
}

func loadGoogleRootCertFromBytes(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("cannot decode embedded Google root certificate: invalid PEM format")
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
