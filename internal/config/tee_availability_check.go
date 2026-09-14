package config

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
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
	// TeeAvailabilityCheck only serves the TEE source. Preflight SOURCE_ID so a
	// mismatched source fails fast at boot with a clear message, instead of booting
	// clean and then rejecting every request with a 400 source-id mismatch.
	if envConfig.SourceID != SourceTEE {
		return nil, fmt.Errorf("unsupported SOURCE_ID %q for TeeAvailabilityCheck: expected %q", envConfig.SourceID, SourceTEE)
	}
	err := CheckMissingFields(envConfig, []string{
		EnvRelayContractAddress,
		EnvFlareRPCURL,
	})
	if err != nil {
		return nil, err
	}
	relayAddr, err := parseContractAddress(envConfig.RelayContractAddress, EnvRelayContractAddress)
	if err != nil {
		return nil, err
	}
	cutoverRelayAddr, cutoverStartingEpoch, err := parseRelayCutover(envConfig, relayAddr)
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

	// CHAIN_ID is required unconditionally: the chain pin in Verify runs even under
	// the E2E/MagicPass bypasses (those disable Google attestation, not chain
	// identity), and 0 is not a valid EVM chain ID, so there is no usable default.
	if envConfig.ChainID == "" {
		return nil, fmt.Errorf("missing environment variables: %s", EnvChainID)
	}
	chainID, err := strconv.ParseUint(envConfig.ChainID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be a base-10 uint64: %q: %w", EnvChainID, envConfig.ChainID, err)
	}
	if chainID == 0 {
		return nil, fmt.Errorf("%s must be non-zero", EnvChainID)
	}

	// TEE_AUDIENCE is an optional override, not a required per-deployment var: the
	// expected aud is the constant tee-node requests its token for, so default to it
	// when unset. Operators only set TEE_AUDIENCE if tee-node's audience diverges.
	teeAudience := envConfig.TeeAudience
	if teeAudience == "" {
		teeAudience = DefaultTeeAudience
	}

	return &TeeAvailabilityCheckConfig{
		EncodedAndABI:                   commonConfig,
		RelayContractAddress:            relayAddr,
		RelayCutoverContractAddress:     cutoverRelayAddr,
		RelayCutoverStartingRewardEpoch: cutoverStartingEpoch,
		AllowTeeDebug:                   allowTeeDebug,
		DisableAttestationCheckE2E:      disableAttestationCheckE2E,
		AllowPrivateNetworks:            allowPrivateNetworks,
		FlareRPCURL:                     envConfig.FlareRPCURL,
		GoogleRootCertificate:           googleRootCert,
		TeeAudience:                     teeAudience,
		ChainID:                         chainID,
	}, nil
}

// parseRelayCutover parses the optional Relay-cutover pair: the redeployed
// Relay's address and the first reward epoch it serves (a reward epoch and a
// signing-policy id are the same identifier; the epoch must match the
// [relay_cutover] blocks of the other Flare clients). Deployed in advance of a
// known Relay redeployment so lookups switch at the configured epoch with no
// redeploy or restart. Both settings or neither — half a cutover would either
// keep every lookup on the old contract or route history to a contract that
// may not carry it. The next Relay is NOT probed at startup: requiring it to
// answer would defeat deploying ahead of the switch; operators must have it
// deployed and initialized before the first new epoch.
func parseRelayCutover(envConfig EnvConfig, relayAddr common.Address) (common.Address, uint32, error) {
	if envConfig.RelayCutoverContractAddress == "" && envConfig.RelayCutoverStartingRewardEpoch == "" {
		return common.Address{}, 0, nil
	}
	if envConfig.RelayCutoverContractAddress == "" || envConfig.RelayCutoverStartingRewardEpoch == "" {
		return common.Address{}, 0, fmt.Errorf("%s and %s must be set together",
			EnvRelayCutoverContractAddress, EnvRelayCutoverStartingRewardEpoch)
	}
	cutoverRelayAddr, err := parseContractAddress(envConfig.RelayCutoverContractAddress, EnvRelayCutoverContractAddress)
	if err != nil {
		return common.Address{}, 0, err
	}
	// Equal addresses would make the cutover a configured no-op; a single
	// Relay is expressed by RELAY_CONTRACT_ADDRESS alone.
	if cutoverRelayAddr == relayAddr {
		return common.Address{}, 0, fmt.Errorf("%s must differ from %s",
			EnvRelayCutoverContractAddress, EnvRelayContractAddress)
	}
	// Reward-epoch ids are uint24 on chain (the Relay contract packs them so):
	// a wider cutoff would boot but could never be reached, so the switch
	// would silently never happen.
	startingRewardEpoch, err := strconv.ParseUint(envConfig.RelayCutoverStartingRewardEpoch, 10, 24)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("%s must be a base-10 uint24 (reward-epoch ids are 24-bit on chain): %q: %w",
			EnvRelayCutoverStartingRewardEpoch, envConfig.RelayCutoverStartingRewardEpoch, err)
	}
	// 0 would route every id to the next Relay — a single-relay setup
	// mis-expressed as a cutover.
	if startingRewardEpoch == 0 {
		return common.Address{}, 0, fmt.Errorf("%s must be non-zero", EnvRelayCutoverStartingRewardEpoch)
	}
	return cutoverRelayAddr, uint32(startingRewardEpoch), nil
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

// ClearTeeAvailabilityCheckConfigForTest resets the TEE availability check config for tests.
func ClearTeeAvailabilityCheckConfigForTest() {
	teeAvailabilityCheckConfig = nil
	errTeeAvailabilityCheckConfig = nil
	teeAvailabilityCheckConfigOnce = sync.Once{}
}
