package paymentservice

import (
	"fmt"
	"io"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	pmwpaymentstatusverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
	"gorm.io/gorm"
)

type PaymentService struct {
	verifier verifierinterface.VerifierInterface[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody,
	]
	config *config.PMWPaymentStatusConfig
	db     *gorm.DB
	cdb    *gorm.DB
}

func NewPaymentService(envConfig config.EnvConfig) (*PaymentService, error) {
	cfg, err := pmwpaymentstatusconfig.GetPMWPaymentStatusConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load PMWPaymentStatus config: %w", err)
	}
	db, err := config.InitMainDB(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DB: %w", err)
	}
	cchainDB, err := config.InitCChainDB(cfg.CchainDatabaseURL)
	if err != nil {
		_ = config.CloseGormDB(db)
		return nil, fmt.Errorf("failed to connect to CChain DB: %w", err)
	}
	verifierImpl, err := pmwpaymentstatusverifier.GetVerifier(cfg, db, cchainDB)
	if err != nil {
		_ = config.CloseGormDB(db)
		_ = config.CloseGormDB(cchainDB)
		return nil, fmt.Errorf("failed to initialize verifier: %w", err)
	}
	return &PaymentService{verifier: verifierImpl, config: cfg, db: db, cdb: cchainDB}, nil
}

func (s *PaymentService) GetVerifier() verifierinterface.VerifierInterface[
	connector.IPMWPaymentStatusRequestBody,
	connector.IPMWPaymentStatusResponseBody,
] {
	return s.verifier
}

func (s *PaymentService) GetConfig() *config.PMWPaymentStatusConfig {
	return s.config
}

func (s *PaymentService) Close() error {
	var errs []error
	if err := config.CloseGormDB(s.db); err != nil {
		errs = append(errs, err)
	}
	if err := config.CloseGormDB(s.cdb); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing PaymentService: %v", errs)
	}
	return nil
}

var _ io.Closer = (*PaymentService)(nil)
