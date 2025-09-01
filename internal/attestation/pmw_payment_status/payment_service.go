package paymentservice

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwstatuspaymentconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	pmwpaymentstatusverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

type PaymentService struct {
	verifier verifierinterface.VerifierInterface[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody,
	]
	config *config.PMWPaymentStatusConfig
}

func NewPaymentService(envConfig config.EnvConfig) (*PaymentService, error) {
	cfg, err := pmwstatuspaymentconfig.GetPMWPaymentStatusConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load PMWPaymentStatus config: %w", err)
	}
	db, err := config.InitMainDB(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DB: %w", err)
	}
	cchainDB, err := config.InitCChainDB(cfg.CchainDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CChain DB: %w", err)
	}
	verifierImpl, err := pmwpaymentstatusverifier.GetVerifier(cfg, db, cchainDB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verifier: %w", err)
	}
	return &PaymentService{verifier: verifierImpl, config: cfg}, nil
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
