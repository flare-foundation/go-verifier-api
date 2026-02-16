package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
)

var (
	pmwPaymentStatusConfig     *PMWPaymentStatusConfig
	pmwPaymentStatusConfigOnce sync.Once
	errPmwPaymentStatusConfig  error
)

func GetPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
	pmwPaymentStatusConfigOnce.Do(func() {
		pmwPaymentStatusConfig, errPmwPaymentStatusConfig = LoadPMWPaymentStatusConfig(envConfig)
	})
	return pmwPaymentStatusConfig, errPmwPaymentStatusConfig
}

func LoadPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
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
	parsedPaymentABI, err := abi.JSON(strings.NewReader(payment.PaymentMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("cannot parse Payment ABI: %w", err)
	}
	return &PMWPaymentStatusConfig{
		EncodedAndABI:            commonConfig,
		SourceDatabaseURL:        envConfig.SourceDatabaseURL,
		CchainDatabaseURL:        envConfig.CChainDatabaseURL,
		ParsedTeeInstructionsABI: parsedTeeInstructionsABI,
		ParsedPaymentABI:         parsedPaymentABI,
	}, nil
}

func ClearPMWPaymentStatusConfigForTest() {
	pmwPaymentStatusConfig = nil
	pmwPaymentStatusConfigOnce = sync.Once{}
	errPmwPaymentStatusConfig = nil
}
