package multisigservice

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	types "github.com/flare-foundation/go-verifier-api/internal/api/type"
	pmwmultisigconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/config"
	pmwmulstisigaccountverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/verifier"
	config "github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

type MultisigService struct {
	verifier verifierinterface.VerifierInterface[
		types.PMWMultisigAccountRequestBody,
		types.PMWMultisigAccountResponseBody,
	]
	config *config.PMWMultisigAccountConfig
}

func NewPaymentService(sourceId config.SourceName, attestationType connector.AttestationType) (*MultisigService, error) {
	cfg, err := pmwmultisigconfig.GetPMWMultisigAccountConfig(sourceId, attestationType)
	if err != nil {
		return nil, fmt.Errorf("failed to load PMWPaymentStatus config: %w", err)
	}
	verifierImpl, err := pmwmulstisigaccountverifier.GetVerifier(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verifier: %w", err)
	}
	return &MultisigService{verifier: verifierImpl, config: cfg}, nil
}

func (s *MultisigService) GetVerifier() verifierinterface.VerifierInterface[
	types.PMWMultisigAccountRequestBody,
	types.PMWMultisigAccountResponseBody,
] {
	return s.verifier
}

func (s *MultisigService) GetConfig() *config.PMWMultisigAccountConfig {
	return s.config
}
