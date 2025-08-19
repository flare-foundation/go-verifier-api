package multisigservice

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwmultisigaccountconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/config"
	pmwmultisigaccountverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

type MultisigService struct {
	verifier verifierinterface.VerifierInterface[
		connector.IPMWMultisigAccountConfiguredRequestBody,
		connector.IPMWMultisigAccountConfiguredResponseBody,
	]
	config *config.PMWMultisigAccountConfig
}

func NewMultisigService(sourceId config.SourceName, attestationType connector.AttestationType) (*MultisigService, error) {
	cfg, err := pmwmultisigaccountconfig.GetPMWMultisigAccountConfig(sourceId, attestationType)
	if err != nil {
		return nil, fmt.Errorf("failed to load PMWPaymentStatus config: %w", err)
	}
	verifierImpl, err := pmwmultisigaccountverifier.GetVerifier(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verifier: %w", err)
	}
	return &MultisigService{verifier: verifierImpl, config: cfg}, nil
}

func (s *MultisigService) GetVerifier() verifierinterface.VerifierInterface[
	connector.IPMWMultisigAccountConfiguredRequestBody,
	connector.IPMWMultisigAccountConfiguredResponseBody,
] {
	return s.verifier
}

func (s *MultisigService) GetConfig() *config.PMWMultisigAccountConfig {
	return s.config
}
