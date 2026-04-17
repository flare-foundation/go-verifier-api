package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
)

var (
	pmwFeeProofConfig     *PMWFeeProofConfig
	pmwFeeProofConfigOnce sync.Once
	errPmwFeeProofConfig  error
)

func LoadPMWFeeProofConfig(envConfig EnvConfig) (*PMWFeeProofConfig, error) {
	pmwFeeProofConfigOnce.Do(func() {
		pmwFeeProofConfig, errPmwFeeProofConfig = BuildPMWFeeProofConfig(envConfig)
	})
	return pmwFeeProofConfig, errPmwFeeProofConfig
}

func BuildPMWFeeProofConfig(envConfig EnvConfig) (*PMWFeeProofConfig, error) {
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
	return &PMWFeeProofConfig{
		EncodedAndABI:                  commonConfig,
		SourceDatabaseURL:              envConfig.SourceDatabaseURL,
		CchainDatabaseURL:              envConfig.CChainDatabaseURL,
		TeeInstructionsContractAddress: teeInstructionsAddr,
		ParsedTeeInstructionsABI:       parsedTeeInstructionsABI,
	}, nil
}

func ClearPMWFeeProofConfigForTest() {
	pmwFeeProofConfig = nil
	pmwFeeProofConfigOnce = sync.Once{}
	errPmwFeeProofConfig = nil
}
