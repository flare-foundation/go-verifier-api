package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/repo"
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
	return &XRPVerifier{
		repo:   repo.NewXRPRepository(db, cChainDB),
		config: cfg,
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
	sourceIdStr := string(cfg.SourceIdPair.SourceId)
	constructor, ok := registry[sourceIdStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIdStr)
	}
	return constructor(cfg, db, cChainDB)
}
