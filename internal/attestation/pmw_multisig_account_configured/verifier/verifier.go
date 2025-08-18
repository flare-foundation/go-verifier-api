package verifier

import (
	"fmt"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	config "github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

type VerifierConstructor func(
	cfg *config.PMWMultisigAccountConfig,
) (verifierinterface.VerifierInterface[attestationtypes.PMWMultisigAccountRequestBody, attestationtypes.PMWMultisigAccountResponseBody], error)

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP): func(cfg *config.PMWMultisigAccountConfig) (
		verifierinterface.VerifierInterface[attestationtypes.PMWMultisigAccountRequestBody, attestationtypes.PMWMultisigAccountResponseBody], error,
	) {
		return &XRPVerifier{config: cfg}, nil
	},
}

func GetVerifier(cfg *config.PMWMultisigAccountConfig) (
	verifierinterface.VerifierInterface[attestationtypes.PMWMultisigAccountRequestBody, attestationtypes.PMWMultisigAccountResponseBody], error,
) {
	sourceIdStr := string(cfg.SourcePair.SourceId)
	constructor, ok := registry[sourceIdStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIdStr)
	}
	return constructor(cfg)
}
