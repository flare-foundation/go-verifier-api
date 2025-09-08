package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

type VerifierConstructor func(
	cfg *config.PMWMultisigAccountConfig,
) (verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody], error)

var xrpConstructor = func(cfg *config.PMWMultisigAccountConfig) (
	verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody], error,
) {
	client := NewXrpClient(cfg.RPCURL, config.ChainRequestRetries, config.ChainRequestTimeout)
	return &XRPVerifier{config: cfg, client: client}, nil
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP): xrpConstructor,
}

func GetVerifier(cfg *config.PMWMultisigAccountConfig) (
	verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody], error,
) {
	sourceIDStr := string(cfg.SourceIDPair.SourceID)
	constructor, ok := registry[sourceIDStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIDStr)
	}
	return constructor(cfg)
}
