package feeproofservice

import (
	"errors"
	"fmt"
	"io"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/db"
	feeproofverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_fee_proof/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type FeeProofService struct {
	verifier attestation.Verifier[
		connector.IPMWFeeProofRequestBody,
		connector.IPMWFeeProofResponseBody,
	]
	config *config.PMWFeeProofConfig
	db     *gorm.DB
	cdb    *gorm.DB
}

func NewFeeProofService(envConfig config.EnvConfig) (*FeeProofService, error) {
	cfg, err := config.GetPMWFeeProofConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load PMWFeeProof config: %w", err)
	}
	dataBase, err := paymentdb.InitSourceDB(cfg.SourceDatabaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Source DB: %w", err)
	}
	cchainDB, err := paymentdb.InitCChainDB(cfg.CchainDatabaseURL, nil)
	if err != nil {
		_ = paymentdb.CloseDB(dataBase)
		return nil, fmt.Errorf("cannot connect to CChain DB: %w", err)
	}
	verifierImpl, err := feeproofverifier.GetVerifier(cfg, dataBase, cchainDB)
	if err != nil {
		_ = paymentdb.CloseDB(dataBase)
		_ = paymentdb.CloseDB(cchainDB)
		return nil, fmt.Errorf("cannot initialize PMWFeeProof verifier: %w", err)
	}
	return &FeeProofService{verifier: verifierImpl, config: cfg, db: dataBase, cdb: cchainDB}, nil
}

func (s *FeeProofService) GetVerifier() attestation.Verifier[
	connector.IPMWFeeProofRequestBody,
	connector.IPMWFeeProofResponseBody,
] {
	return s.verifier
}

func (s *FeeProofService) GetConfig() *config.PMWFeeProofConfig {
	return s.config
}

func (s *FeeProofService) Close() error {
	var errs []error
	if err := paymentdb.CloseDB(s.db); err != nil {
		errs = append(errs, err)
	}
	if err := paymentdb.CloseDB(s.cdb); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing FeeProofService: %w", errors.Join(errs...))
	}
	return nil
}

// Ensure *FeeProofService implements io.Closer at compile time.
var _ io.Closer = (*FeeProofService)(nil)
