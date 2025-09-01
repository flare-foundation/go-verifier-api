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
	client := NewXrpClient(cfg.RPCURL)
	return &XRPVerifier{config: cfg, client: client}, nil
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP):     xrpConstructor,
	string(config.SourceTestXRP): xrpConstructor,
}

func GetVerifier(cfg *config.PMWMultisigAccountConfig) (
	verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody], error,
) {
	sourceIdStr := string(cfg.EncodedAndAbi.SourceIdPair.SourceId)
	constructor, ok := registry[sourceIdStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIdStr)
	}
	return constructor(cfg)
}
