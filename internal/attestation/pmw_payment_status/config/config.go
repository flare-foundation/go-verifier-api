package pmwpaymentstatusconfig

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeinstructions"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payment"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	pmyPaymentStatusConfig     *config.PMWPaymentStatusConfig
	pmyPaymentStatusConfigOnce sync.Once
	pmyPaymentStatusConfigErr  error
)

func GetPMWPaymentStatusConfig(sourceId config.SourceName, attestationType connector.AttestationType) (*config.PMWPaymentStatusConfig, error) {
	pmyPaymentStatusConfigOnce.Do(func() {
		pmyPaymentStatusConfig, pmyPaymentStatusConfigErr = LoadPMWPaymentStatusConfig(sourceId, attestationType)
	})
	return pmyPaymentStatusConfig, pmyPaymentStatusConfigErr
}

func LoadPMWPaymentStatusConfig(sourceId config.SourceName, attestationType connector.AttestationType) (*config.PMWPaymentStatusConfig, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}
	cChainDbURL := os.Getenv("CCHAIN_DATABASE_URL")
	if cChainDbURL == "" {
		return nil, fmt.Errorf("CCHAIN_DATABASE_URL not set")
	}
	commonConfig, err := config.LoadEncodedAndAbi(sourceId, attestationType)
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
		DatabaseURL:              dbURL,
		CchainDatabaseURL:        cChainDbURL,
		AttestationTypePair:      commonConfig.AttestationTypePair,
		AbiPair:                  commonConfig.AbiPair,
		ParsedTeeInstructionsABI: parsedTeeInstructionsABI,
		ParsedPaymentABI:         parsedPaymentABI,
	}, nil
}
