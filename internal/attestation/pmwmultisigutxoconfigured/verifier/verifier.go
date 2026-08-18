package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"

	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	btcverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigutxoconfigured/btc"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

type VerifierConstructor func(
	cfg *config.PMWMultisigUtxoConfig,
) (attestation.Verifier[fdc2.IPMWMultisigUtxoConfiguredRequestBody, fdc2.IPMWMultisigUtxoConfiguredResponseBody], error)

var btcConstructor = func(cfg *config.PMWMultisigUtxoConfig) (
	attestation.Verifier[fdc2.IPMWMultisigUtxoConfiguredRequestBody, fdc2.IPMWMultisigUtxoConfiguredResponseBody], error,
) {
	return btcverifier.NewBtcVerifier(cfg)
}

var registry = map[string]VerifierConstructor{
	string(config.SourceBTC):     btcConstructor,
	string(config.SourceTestBTC): btcConstructor,
}

func NewVerifier(cfg *config.PMWMultisigUtxoConfig) (
	attestation.Verifier[fdc2.IPMWMultisigUtxoConfiguredRequestBody, fdc2.IPMWMultisigUtxoConfiguredResponseBody], error,
) {
	sourceIDStr := string(cfg.SourceIDPair.SourceID)
	constructor, ok := registry[sourceIDStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIDStr)
	}
	return constructor(cfg)
}
