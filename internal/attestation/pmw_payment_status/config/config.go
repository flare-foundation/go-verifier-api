package pmwpaymentstatusconfig

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeinstructions"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	pmyPaymentStatusConfig     *config.PMWPaymentStatusConfig
	pmyPaymentStatusConfigOnce sync.Once
	pmyPaymentStatusConfigErr  error
)

func GetPMWPaymentStatusConfig(envConfig config.EnvConfig) (*config.PMWPaymentStatusConfig, error) {
	pmyPaymentStatusConfigOnce.Do(func() {
		pmyPaymentStatusConfig, pmyPaymentStatusConfigErr = LoadPMWPaymentStatusConfig(envConfig)
	})
	return pmyPaymentStatusConfig, pmyPaymentStatusConfigErr
}

func LoadPMWPaymentStatusConfig(envConfig config.EnvConfig) (*config.PMWPaymentStatusConfig, error) {
	if envConfig.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}
	if envConfig.CChainDatabaseURL == "" {
		return nil, fmt.Errorf("CCHAIN_DATABASE_URL not set")
	}
	commonConfig, err := config.LoadEncodedAndAbi(envConfig)
	if err != nil {
		return nil, err
	}
	parsedTeeInstructionsABI, err := abi.JSON(strings.NewReader(teeinstructions.TeeInstructionsABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse TeeInstructions ABI: %w", err)
	}
	parsedPaymentABI, err := abi.JSON(strings.NewReader(payment.PaymentMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Payment ABI: %w", err)
	}
	return &config.PMWPaymentStatusConfig{
		SourcePair:               commonConfig.SourceIdPair,
		DatabaseURL:              envConfig.DatabaseURL,
		CchainDatabaseURL:        envConfig.CChainDatabaseURL,
		AttestationTypePair:      commonConfig.AttestationTypePair,
		AbiPair:                  commonConfig.AbiPair,
		ParsedTeeInstructionsABI: parsedTeeInstructionsABI,
		ParsedPaymentABI:         parsedPaymentABI,
	}, nil
}
