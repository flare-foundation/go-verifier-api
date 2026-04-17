package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
)

var (
	pmwPaymentStatusConfig     *PMWPaymentStatusConfig
	pmwPaymentStatusConfigOnce sync.Once
	errPmwPaymentStatusConfig  error
)

func LoadPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
	pmwPaymentStatusConfigOnce.Do(func() {
		pmwPaymentStatusConfig, errPmwPaymentStatusConfig = BuildPMWPaymentStatusConfig(envConfig)
	})
	return pmwPaymentStatusConfig, errPmwPaymentStatusConfig
}

func BuildPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
	err := CheckMissingFields(envConfig, []string{EnvCChainDatabaseURL, EnvSourceDatabaseURL, EnvTeeInstructionsContractAddress})
	if err != nil {
		return nil, err
	}
	teeInstructionsAddr, err := parseContractAddress(envConfig.TeeInstructionsContractAddress, EnvTeeInstructionsContractAddress)
	if err != nil {
		return nil, err
	}
	commonConfig, err := LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	parsedTeeInstructionsABI, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("cannot parse TeeInstructions ABI: %w", err)
	}
	return &PMWPaymentStatusConfig{
		EncodedAndABI:                  commonConfig,
		SourceDatabaseURL:              envConfig.SourceDatabaseURL,
		CchainDatabaseURL:              envConfig.CChainDatabaseURL,
		TeeInstructionsContractAddress: teeInstructionsAddr,
		ParsedTeeInstructionsABI:       parsedTeeInstructionsABI,
	}, nil
}

// parseContractAddress validates a 0x-prefixed hex address and rejects zero/malformed values.
func parseContractAddress(raw, envName string) (common.Address, error) {
	if !common.IsHexAddress(raw) {
		return common.Address{}, fmt.Errorf("%s is not a valid hex address: %q", envName, raw)
	}
	addr := common.HexToAddress(raw)
	if addr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("%s must not be the zero address", envName)
	}
	return addr, nil
}

func ClearPMWPaymentStatusConfigForTest() {
	pmwPaymentStatusConfig = nil
	pmwPaymentStatusConfigOnce = sync.Once{}
	errPmwPaymentStatusConfig = nil
}
