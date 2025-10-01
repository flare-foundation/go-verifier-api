package paymentservice

import (
	"fmt"
	"io"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	pmwpaymentstatusverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/verifier"
	mainconfig "github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
	"gorm.io/gorm"
)

type PaymentService struct {
	verifier verifierinterface.VerifierInterface[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody,
	]
	config *mainconfig.PMWPaymentStatusConfig
	db     *gorm.DB
	cdb    *gorm.DB
}

func NewPaymentService(envConfig mainconfig.EnvConfig) (*PaymentService, error) {
	cfg, err := config.GetPMWPaymentStatusConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load PMWPaymentStatus config: %w", err)
	}
	db, err := config.InitSourceDB(cfg.SourceDatabaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Source DB: %w", err)
	}
	cchainDB, err := config.InitCChainDB(cfg.CchainDatabaseURL, nil)
	if err != nil {
		_ = config.CloseDB(db)
		return nil, fmt.Errorf("failed to connect to CChain DB: %w", err)
	}
	verifierImpl, err := pmwpaymentstatusverifier.GetVerifier(cfg, db, cchainDB)
	if err != nil {
		_ = config.CloseDB(db)
		_ = config.CloseDB(cchainDB)
		return nil, fmt.Errorf("failed to initialize PMWPaymentStatus verifier: %w", err)
	}
	return &PaymentService{verifier: verifierImpl, config: cfg, db: db, cdb: cchainDB}, nil
}

func (s *PaymentService) GetVerifier() verifierinterface.VerifierInterface[
	connector.IPMWPaymentStatusRequestBody,
	connector.IPMWPaymentStatusResponseBody,
] {
	return s.verifier
}

func (s *PaymentService) GetConfig() *mainconfig.PMWPaymentStatusConfig {
	return s.config
}

func (s *PaymentService) Close() error {
	var errs []error
	if err := config.CloseDB(s.db); err != nil {
		errs = append(errs, err)
	}
	if err := config.CloseDB(s.cdb); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing PaymentService: %v", errs)
	}
	return nil
}

var _ io.Closer = (*PaymentService)(nil)
