package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	xrpverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

// VerifierConstructor builds a verifier once the source is resolved and the DB
// connections are open. Resolving the source (ConstructorForSource) is fallible;
// construction itself is not.
type VerifierConstructor func(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) attestation.Verifier[fdc2.IPMWPaymentStatusRequestBody, fdc2.IPMWPaymentStatusResponseBody]

var xrpConstructor = func(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) attestation.Verifier[fdc2.IPMWPaymentStatusRequestBody, fdc2.IPMWPaymentStatusResponseBody] {
	return xrpverifier.NewXRPVerifier(cfg, db, cChainDB)
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP):     xrpConstructor,
	string(config.SourceTestXRP): xrpConstructor,
}

// ConstructorForSource returns the verifier constructor for the given source ID,
// or an error if the source is unsupported. Services resolve this BEFORE opening
// DB connections, so a misconfigured SOURCE_ID fails fast with a clear error
// instead of a DB failure; the returned constructor then cannot fail.
func ConstructorForSource(sourceID string) (VerifierConstructor, error) {
	constructor, ok := registry[sourceID]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceID)
	}
	return constructor, nil
}
