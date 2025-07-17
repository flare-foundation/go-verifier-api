package verifier

import (
	"fmt"

	attestationtypes "gitlab.com/urskak/verifier-api/internal/api/type"
	pmwpaymentstatusconfig "gitlab.com/urskak/verifier-api/internal/attestation/pmw_payment_status/config"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
	"gorm.io/gorm"
)

type VerifierConstructor func(
	cfg *pmwpaymentstatusconfig.PMWPaymentStatusConfig,
	db *gorm.DB,
	cChainDB *gorm.DB,
) (verifierinterface.VerifierInterface[attestationtypes.IPMWPaymentStatusRequestBody, attestationtypes.IPMWPaymentStatusResponseBody], error)

var registry = map[string]VerifierConstructor{
	string(attestationtypes.SourceXRP): func(cfg *pmwpaymentstatusconfig.PMWPaymentStatusConfig, db *gorm.DB, cChainDB *gorm.DB) (
		verifierinterface.VerifierInterface[attestationtypes.IPMWPaymentStatusRequestBody, attestationtypes.IPMWPaymentStatusResponseBody], error,
	) {
		return &XRPVerifier{db: db, cChainDb: cChainDB, config: cfg}, nil
	},
}

func GetVerifier(sourceID string, cfg *pmwpaymentstatusconfig.PMWPaymentStatusConfig, db, cChainDB *gorm.DB) (
	verifierinterface.VerifierInterface[attestationtypes.IPMWPaymentStatusRequestBody, attestationtypes.IPMWPaymentStatusResponseBody], error,
) {
	constructor, ok := registry[sourceID]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceID)
	}
	return constructor(cfg, db, cChainDB)
}
