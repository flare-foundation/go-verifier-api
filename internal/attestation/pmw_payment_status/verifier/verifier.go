package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	xrpverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

type VerifierConstructor func(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) (attestation.Verifier[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error)

var xrpConstructor = func(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) (
	attestation.Verifier[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error,
) {
	return xrpverifier.NewXRPVerifier(cfg, db, cChainDB), nil
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP):     xrpConstructor,
	string(config.SourceTestXRP): xrpConstructor,
}

func NewVerifier(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) (
	attestation.Verifier[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error,
) {
	sourceIDStr := string(cfg.SourceIDPair.SourceID)
	constructor, ok := registry[sourceIDStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIDStr)
	}
	return constructor(cfg, db, cChainDB)
}
