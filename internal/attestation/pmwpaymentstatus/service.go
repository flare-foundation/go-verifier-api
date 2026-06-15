package paymentservice

import (
	"errors"
	"fmt"
	"io"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	pmwpaymentstatusverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type PaymentService struct {
	verifier attestation.Verifier[
		fdc2.IPMWPaymentStatusRequestBody,
		fdc2.IPMWPaymentStatusResponseBody,
	]
	config *config.PMWPaymentStatusConfig
	db     *gorm.DB
	cdb    *gorm.DB
}

func NewPaymentService(envConfig config.EnvConfig) (*PaymentService, error) {
	cfg, err := config.LoadPMWPaymentStatusConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load PMWPaymentStatus config: %w", err)
	}
	// Resolve the source before opening DB connections so a misconfigured
	// SOURCE_ID fails fast with a clear error instead of a DB failure.
	construct, err := pmwpaymentstatusverifier.ConstructorForSource(string(cfg.SourceIDPair.SourceID))
	if err != nil {
		return nil, fmt.Errorf("unsupported SOURCE_ID %q for PMWPaymentStatus: %w", cfg.SourceIDPair.SourceID, err)
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
	// Source already resolved above, so construction cannot fail.
	verifierImpl := construct(cfg, dataBase, cchainDB)
	return &PaymentService{verifier: verifierImpl, config: cfg, db: dataBase, cdb: cchainDB}, nil
}

func (s *PaymentService) Verifier() attestation.Verifier[
	fdc2.IPMWPaymentStatusRequestBody,
	fdc2.IPMWPaymentStatusResponseBody,
] {
	return s.verifier
}

func (s *PaymentService) Config() *config.PMWPaymentStatusConfig {
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
