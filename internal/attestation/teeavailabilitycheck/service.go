package teeavailabilityservice

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	teeavailabilitycheck "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

type TeeAvailabilityService struct {
	verifier attestation.Verifier[connector.ITeeAvailabilityCheckRequestBody, connector.ITeeAvailabilityCheckResponseBody]
	config   *config.TeeAvailabilityCheckConfig
}

func NewTeeAvailabilityService(envConfig config.EnvConfig) (*TeeAvailabilityService, error) {
	cfg, err := config.LoadTeeAvailabilityCheckConfig(envConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load TeeAvailabilityCheck config: %w", err)
	}
	verifier, err := teeavailabilitycheck.NewVerifier(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize TeeAvailabilityCheck verifier: %w", err)
	}

	return &TeeAvailabilityService{verifier: verifier, config: cfg}, nil
}

func (s *TeeAvailabilityService) Verifier() attestation.Verifier[
	connector.ITeeAvailabilityCheckRequestBody,
	connector.ITeeAvailabilityCheckResponseBody,
] {
	return s.verifier
}

func (s *TeeAvailabilityService) Config() *config.TeeAvailabilityCheckConfig {
	return s.config
}
