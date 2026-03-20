package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	xrpverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_fee_proof/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type VerifierConstructor func(
	cfg *config.PMWFeeProofConfig,
	db, cChainDB *gorm.DB,
) (attestation.Verifier[connector.IPMWFeeProofRequestBody, connector.IPMWFeeProofResponseBody], error)

var xrpConstructor = func(
	cfg *config.PMWFeeProofConfig,
	db, cChainDB *gorm.DB,
) (
	attestation.Verifier[connector.IPMWFeeProofRequestBody, connector.IPMWFeeProofResponseBody], error,
) {
	return xrpverifier.NewXRPVerifier(cfg, db, cChainDB), nil
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP):     xrpConstructor,
	string(config.SourceTestXRP): xrpConstructor,
}

func GetVerifier(
	cfg *config.PMWFeeProofConfig,
	db, cChainDB *gorm.DB,
) (
	attestation.Verifier[connector.IPMWFeeProofRequestBody, connector.IPMWFeeProofResponseBody], error,
) {
	sourceIDStr := string(cfg.SourceIDPair.SourceID)
	constructor, ok := registry[sourceIDStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIDStr)
	}
	return constructor(cfg, db, cChainDB)
}
