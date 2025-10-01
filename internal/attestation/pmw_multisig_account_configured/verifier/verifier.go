package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	xrpverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_multisig_account_configured/xrp/client"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
)

type VerifierConstructor func(
	cfg *config.PMWMultisigAccountConfig,
) (verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody], error)

var xrpConstructor = func(cfg *config.PMWMultisigAccountConfig) (
	verifierinterface.VerifierInterface[connector.IPMWMultisigAccountConfiguredRequestBody, connector.IPMWMultisigAccountConfiguredResponseBody], error,
) {
	client := client.NewClient(cfg.RPCURL)
	return &xrpverifier.XRPVerifier{Config: cfg, Client: client}, nil
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP):     xrpConstructor,
	string(config.SourceTestXRP): xrpConstructor,
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
