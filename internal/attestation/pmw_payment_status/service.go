package paymentservice

import (
	"errors"
	"fmt"
	"io"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/db"
	pmwpaymentstatusverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type PaymentService struct {
	verifier attestation.Verifier[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody,
	]
	config *config.PMWPaymentStatusConfig
	db     *gorm.DB
	cdb    *gorm.DB
}

func NewPaymentService(envConfig config.EnvConfig) (*PaymentService, error) {
	cfg, err := config.GetPMWPaymentStatusConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load PMWPaymentStatus config: %w", err)
	}
	dataBase, err := db.InitSourceDB(cfg.SourceDatabaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Source DB: %w", err)
	}
	cchainDB, err := db.InitCChainDB(cfg.CchainDatabaseURL, nil)
	if err != nil {
		_ = db.CloseDB(dataBase)
		return nil, fmt.Errorf("cannot connect to CChain DB: %w", err)
	}
	verifierImpl, err := pmwpaymentstatusverifier.GetVerifier(cfg, dataBase, cchainDB)
	if err != nil {
		_ = db.CloseDB(dataBase)
		_ = db.CloseDB(cchainDB)
		return nil, fmt.Errorf("cannot initialize PMWPaymentStatus verifier: %w", err)
	}
	return &PaymentService{verifier: verifierImpl, config: cfg, db: dataBase, cdb: cchainDB}, nil
}

func (s *PaymentService) GetVerifier() attestation.Verifier[
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
	if err := db.CloseDB(s.db); err != nil {
		errs = append(errs, err)
	}
	if err := db.CloseDB(s.cdb); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing PaymentService: %w", errors.Join(errs...))
	}
	return nil
}

// Ensure *PaymentService implements io.Closer at compile time.
var _ io.Closer = (*PaymentService)(nil)
