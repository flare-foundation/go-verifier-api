package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	pmwpaymentstatusconfig "gitlab.com/urskak/verifier-api/internal/attestation/pmw_payment_status/config"
	types "gitlab.com/urskak/verifier-api/internal/common"
	verifierinterface "gitlab.com/urskak/verifier-api/internal/verifier_interface"
	"gorm.io/gorm"
)

type VerifierConstructor func(
	cfg *pmwpaymentstatusconfig.PMWPaymentStatusConfig,
	db *gorm.DB,
	cChainDB *gorm.DB,
) (verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error)

var registry = map[string]VerifierConstructor{
	string(types.SourceXRP): func(cfg *pmwpaymentstatusconfig.PMWPaymentStatusConfig, db *gorm.DB, cChainDB *gorm.DB) (
		verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error,
	) {
		return &XRPVerifier{db: db, cChainDb: cChainDB, config: cfg}, nil
	},
}

func GetVerifier(sourceID string, cfg *pmwpaymentstatusconfig.PMWPaymentStatusConfig, db, cChainDB *gorm.DB) (
	verifierinterface.VerifierInterface[connector.IPMWPaymentStatusRequestBody, connector.IPMWPaymentStatusResponseBody], error,
) {
	constructor, ok := registry[sourceID]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceID)
	}
	return constructor(cfg, db, cChainDB)
}
