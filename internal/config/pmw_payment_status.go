package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
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
	err := CheckMissingFields(envConfig, []string{EnvCChainDatabaseURL, EnvSourceDatabaseURL})
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
		EncodedAndABI:            commonConfig,
		SourceDatabaseURL:        envConfig.SourceDatabaseURL,
		CchainDatabaseURL:        envConfig.CChainDatabaseURL,
		ParsedTeeInstructionsABI: parsedTeeInstructionsABI,
	}, nil
}

func ClearPMWPaymentStatusConfigForTest() {
	pmwPaymentStatusConfig = nil
	pmwPaymentStatusConfigOnce = sync.Once{}
	errPmwPaymentStatusConfig = nil
}
