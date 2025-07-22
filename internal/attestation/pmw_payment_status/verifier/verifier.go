package verifier

import (
	"fmt"

	attestationtypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
	config "github.com/flare-foundation/go-verifier-api/internal/config"
	verifierinterface "github.com/flare-foundation/go-verifier-api/internal/verifier_interface"
	"gorm.io/gorm"
)

type VerifierConstructor func(
	cfg *config.PMWPaymentStatusConfig,
	db *gorm.DB,
	cChainDB *gorm.DB,
) (verifierinterface.VerifierInterface[attestationtypes.PMWPaymentStatusRequestBody, attestationtypes.PMWPaymentStatusResponseBody], error)

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP): func(cfg *config.PMWPaymentStatusConfig, db *gorm.DB, cChainDB *gorm.DB) (
		verifierinterface.VerifierInterface[attestationtypes.PMWPaymentStatusRequestBody, attestationtypes.PMWPaymentStatusResponseBody], error,
	) {
		return &XRPVerifier{db: db, cChainDb: cChainDB, config: cfg}, nil
	},
}

func GetVerifier(cfg *config.PMWPaymentStatusConfig, db, cChainDB *gorm.DB) (
	verifierinterface.VerifierInterface[attestationtypes.PMWPaymentStatusRequestBody, attestationtypes.PMWPaymentStatusResponseBody], error,
) {
	sourceIdStr := string(cfg.SourcePair.SourceId)
	constructor, ok := registry[sourceIdStr]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceIdStr)
	}
	return constructor(cfg, db, cChainDB)
}
