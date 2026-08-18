package utxomultisigservice

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	utxoverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigutxoconfigured/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

type UtxoMultisigService struct {
	verifier attestation.Verifier[
		fdc2.IPMWMultisigUtxoConfiguredRequestBody,
		fdc2.IPMWMultisigUtxoConfiguredResponseBody,
	]
	config *config.PMWMultisigUtxoConfig
}

func NewUtxoMultisigService(envConfig config.EnvConfig) (*UtxoMultisigService, error) {
	cfg, err := config.LoadPMWMultisigUtxoConfiguredConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load PMWMultisigUtxoConfigured config: %w", err)
	}
	verifierImpl, err := utxoverifier.NewVerifier(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize PMWMultisigUtxoConfigured verifier: %w", err)
	}
	return &UtxoMultisigService{verifier: verifierImpl, config: cfg}, nil
}

func (s *UtxoMultisigService) Verifier() attestation.Verifier[
	fdc2.IPMWMultisigUtxoConfiguredRequestBody,
	fdc2.IPMWMultisigUtxoConfiguredResponseBody,
] {
	return s.verifier
}

func (s *UtxoMultisigService) Config() *config.PMWMultisigUtxoConfig {
	return s.config
}
