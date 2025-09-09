package pmwpaymentstatusconfig

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	pmyPaymentStatusConfig     *config.PMWPaymentStatusConfig
	pmyPaymentStatusConfigOnce sync.Once
	errPmyPaymentStatusConfig  error
)

func GetPMWPaymentStatusConfig(envConfig config.EnvConfig) (*config.PMWPaymentStatusConfig, error) {
	pmyPaymentStatusConfigOnce.Do(func() {
		pmyPaymentStatusConfig, errPmyPaymentStatusConfig = LoadPMWPaymentStatusConfig(envConfig)
	})
	return pmyPaymentStatusConfig, errPmyPaymentStatusConfig
}

func LoadPMWPaymentStatusConfig(envConfig config.EnvConfig) (*config.PMWPaymentStatusConfig, error) {
	err := config.CheckMissingFields(envConfig, []string{config.EnvCChainDatabaseURL, config.EnvDatabaseURL})
	if err != nil {
		return nil, err
	}
	commonConfig, err := config.LoadEncodedAndAbi(envConfig)
	if err != nil {
		return nil, err
	}
	parsedTeeInstructionsABI, err := abi.JSON(strings.NewReader(teeextensionregistry.TeeExtensionRegistryMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse TeeInstructions ABI: %w", err)
	}
	parsedPaymentABI, err := abi.JSON(strings.NewReader(payment.PaymentMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Payment ABI: %w", err)
	}
	return &config.PMWPaymentStatusConfig{
		EncodedAndAbi:            commonConfig,
		DatabaseURL:              envConfig.DatabaseURL,
		CchainDatabaseURL:        envConfig.CChainDatabaseURL,
		ParsedTeeInstructionsABI: parsedTeeInstructionsABI,
		ParsedPaymentABI:         parsedPaymentABI,
	}, nil
}

func ClearPMWPaymentStatusConfigForTest() {
	pmyPaymentStatusConfig = nil
	pmyPaymentStatusConfigOnce = sync.Once{}
	errPmyPaymentStatusConfig = nil
}
