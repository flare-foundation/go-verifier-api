package paymentservice

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwpaymentstatusverifier "gitlab.com/urskak/verifier-api/internal/attestation/pmw_payment_status/verifier"
	"gitlab.com/urskak/verifier-api/internal/config"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
)

type PaymentService struct {
	verifier verifierinterface.VerifierInterface[
		connector.IPMWPaymentStatusRequestBody,
		connector.IPMWPaymentStatusResponseBody,
	]
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
	return &PaymentService{verifier: verifierImpl}, nil
}

func (s *PaymentService) GetVerifier() verifierinterface.VerifierInterface[
	connector.IPMWPaymentStatusRequestBody,
	connector.IPMWPaymentStatusResponseBody,
] {
	return s.verifier
}
