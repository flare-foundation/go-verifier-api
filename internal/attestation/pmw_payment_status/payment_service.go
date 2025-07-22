package paymentservice

import (
	"fmt"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	pmwpaymentstatusverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

type PaymentService struct {
	verifier verifierinterface.VerifierInterface[
		attestationtypes.PMWPaymentStatusRequestBody,
		attestationtypes.PMWPaymentStatusResponseBody,
	]
	config *pmwpaymentstatusconfig.PMWPaymentStatusConfig
}

func NewPaymentService() (*PaymentService, error) {
	cfg, err := config.GetPMWPaymentStatusConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load PMWPaymentStatus config: %w", err)
	}
	sourceID := cfg.SourceID
	db, err := config.InitMainDB(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DB: %w", err)
	}
	cchainDB, err := config.InitCChainDB(cfg.CchainDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CChain DB: %w", err)
	}
	verifierImpl, err := pmwpaymentstatusverifier.GetVerifier(sourceID, cfg, db, cchainDB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verifier: %w", err)
	}
	return &PaymentService{verifier: verifierImpl, config: cfg}, nil
}

func (s *PaymentService) GetVerifier() verifierinterface.VerifierInterface[
	attestationtypes.PMWPaymentStatusRequestBody,
	attestationtypes.PMWPaymentStatusResponseBody,
] {
	return s.verifier
}

func (s *PaymentService) GetConfig() *pmwpaymentstatusconfig.PMWPaymentStatusConfig {
	return s.config
}
