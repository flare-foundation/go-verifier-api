package feeproofservice

import (
	"errors"
	"fmt"
	"io"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	feeproofverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwfeeproof/verifier"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type FeeProofService struct {
	verifier attestation.Verifier[
		fdc2.IPMWFeeProofRequestBody,
		fdc2.IPMWFeeProofResponseBody,
	]
	config *config.PMWFeeProofConfig
	db     *gorm.DB
	cdb    *gorm.DB
}

func NewFeeProofService(envConfig config.EnvConfig) (*FeeProofService, error) {
	cfg, err := config.LoadPMWFeeProofConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load PMWFeeProof config: %w", err)
	}
	// Resolve the source before opening DB connections so a misconfigured
	// SOURCE_ID fails fast with a clear error instead of a DB failure.
	construct, err := feeproofverifier.ConstructorForSource(string(cfg.SourceIDPair.SourceID))
	if err != nil {
		return nil, fmt.Errorf("unsupported SOURCE_ID %q for PMWFeeProof: %w", cfg.SourceIDPair.SourceID, err)
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
	// Source already resolved above; construction can still fail when dialing the
	// Flare RPC for the on-chain initial-nonce binding.
	verifierImpl, err := construct(cfg, dataBase, cchainDB)
	if err != nil {
		_ = paymentdb.CloseDB(dataBase)
		_ = paymentdb.CloseDB(cchainDB)
		return nil, fmt.Errorf("cannot construct PMWFeeProof verifier: %w", err)
	}
	return &FeeProofService{verifier: verifierImpl, config: cfg, db: dataBase, cdb: cchainDB}, nil
}

func (s *FeeProofService) Verifier() attestation.Verifier[
	fdc2.IPMWFeeProofRequestBody,
	fdc2.IPMWFeeProofResponseBody,
] {
	return s.verifier
}

func (s *FeeProofService) Config() *config.PMWFeeProofConfig {
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
	// The verifier holds the on-chain initial-nonce binder's RPC connection.
	if c, ok := s.verifier.(io.Closer); ok {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing FeeProofService: %w", errors.Join(errs...))
	}
	return nil
}

// Ensure *FeeProofService implements io.Closer at compile time.
var _ io.Closer = (*FeeProofService)(nil)
