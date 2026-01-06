package multisigservice

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	pmwmultisigaccountverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

type MultisigService struct {
	verifier attestation.Verifier[
		connector.IPMWMultisigAccountConfiguredRequestBody,
		connector.IPMWMultisigAccountConfiguredResponseBody,
	]
	config *config.PMWMultisigAccountConfig
}

func NewMultisigService(envConfig config.EnvConfig) (*MultisigService, error) {
	cfg, err := config.GetPMWMultisigAccountConfiguredConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load PMWMultisigAccountConfigured config: %w", err)
	}
	verifierImpl, err := pmwmultisigaccountverifier.GetVerifier(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize PMWMultisigAccountConfigured verifier: %w", err)
	}
	return &MultisigService{verifier: verifierImpl, config: cfg}, nil
}

func (s *MultisigService) GetVerifier() attestation.Verifier[
	connector.IPMWMultisigAccountConfiguredRequestBody,
	connector.IPMWMultisigAccountConfiguredResponseBody,
] {
	return s.verifier
}

func (s *MultisigService) GetConfig() *config.PMWMultisigAccountConfig {
	return s.config
}
