package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"

	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	xrpverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigconfigured/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

type VerifierConstructor func(
	cfg *config.PMWMultisigAccountConfig,
) (attestation.Verifier[fdc2.IPMWMultisigAccountConfiguredRequestBody, fdc2.IPMWMultisigAccountConfiguredResponseBody], error)

var xrpConstructor = func(cfg *config.PMWMultisigAccountConfig) (
	attestation.Verifier[fdc2.IPMWMultisigAccountConfiguredRequestBody, fdc2.IPMWMultisigAccountConfiguredResponseBody], error,
) {
	return xrpverifier.NewXRPVerifier(cfg), nil
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP):     xrpConstructor,
	string(config.SourceTestXRP): xrpConstructor,
}

func NewVerifier(cfg *config.PMWMultisigAccountConfig) (
	attestation.Verifier[fdc2.IPMWMultisigAccountConfiguredRequestBody, fdc2.IPMWMultisigAccountConfiguredResponseBody], error,
) {
	sourceIDStr := string(cfg.SourceIDPair.SourceID)
	constructor, ok := registry[sourceIDStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIDStr)
	}
	return constructor(cfg)
}
