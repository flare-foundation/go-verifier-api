package pmwpaymentstatusconfig

import (
	"fmt"
	"os"
	"sync"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
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
	sourceIdEnc, err := config.EncodeAttestationOrSourceName(string(sourceId))
	if err != nil {
		return nil, err
	}
	attestationTypeEnc, err := config.EncodeAttestationOrSourceName(string(attestationType))
	if err != nil {
		return nil, err
	}
	return &config.PMWPaymentStatusConfig{
		SourcePair:          config.SourceIdEncodedPair{SourceId: sourceId, SourceIdEncoded: sourceIdEnc},
		DatabaseURL:         dbURL,
		CchainDatabaseURL:   cChainDbURL,
		AttestationTypePair: config.AttestationTypeEncodedPair{AttestationType: attestationType, AttestationTypeEncoded: attestationTypeEnc},
	}, nil
}
