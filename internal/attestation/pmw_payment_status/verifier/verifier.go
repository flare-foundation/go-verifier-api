package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	xrpverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/repo"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
	"gorm.io/gorm"
)

type VerifierConstructor func(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) (verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error)

var xrpConstructor = func(cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB) (
	verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error,
) {
	return &xrpverifier.XRPVerifier{
		Repo:   repo.NewXRPRepository(db, cChainDB),
		Config: cfg,
	}, nil
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP): xrpConstructor,
}

func GetVerifier(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB) (
	verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error,
) {
	sourceIDStr := string(cfg.SourceIDPair.SourceID)
	constructor, ok := registry[sourceIDStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIDStr)
	}
	return constructor(cfg, db, cChainDB)
}
