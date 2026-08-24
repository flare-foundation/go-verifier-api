package verifier

import (
	"fmt"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/attestation"
	btcverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc"
	xrpverifier "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/xrp"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"gorm.io/gorm"
)

// VerifierConstructor builds a verifier once the source is resolved and the DB
// connections are open. Resolving the source (ConstructorForSource) is fallible,
// and so is construction itself — it dials the Flare RPC for the on-chain
// initial-nonce binding.
type VerifierConstructor func(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) (attestation.Verifier[fdc2.IPMWPaymentStatusRequestBody, fdc2.IPMWPaymentStatusResponseBody], error)

var xrpConstructor = func(
	cfg *config.PMWPaymentStatusConfig,
	db, cChainDB *gorm.DB,
) (attestation.Verifier[fdc2.IPMWPaymentStatusRequestBody, fdc2.IPMWPaymentStatusResponseBody], error) {
	return xrpverifier.NewXRPVerifier(cfg, db, cChainDB)
}

// btcConstructor builds the BTC (node-path) verifier. The source DB is the
// verifier-utxo-indexer, which the node path does not use, so the first *gorm.DB
// is ignored; only the C-chain DB (PaymentBatched events) is passed through.
var btcConstructor = func(
	cfg *config.PMWPaymentStatusConfig,
	_, cChainDB *gorm.DB,
) (attestation.Verifier[fdc2.IPMWPaymentStatusRequestBody, fdc2.IPMWPaymentStatusResponseBody], error) {
	return btcverifier.NewBtcVerifier(cfg, cChainDB)
}

var registry = map[string]VerifierConstructor{
	string(config.SourceXRP):     xrpConstructor,
	string(config.SourceTestXRP): xrpConstructor,
	string(config.SourceBTC):     btcConstructor,
	string(config.SourceTestBTC): btcConstructor,
}

// ConstructorForSource returns the verifier constructor for the given source ID,
// or an error if the source is unsupported. Services resolve this BEFORE opening
// DB connections, so a misconfigured SOURCE_ID fails fast with a clear error
// instead of a DB failure. The returned constructor can still fail (it dials the
// Flare RPC for the on-chain initial-nonce binder).
func ConstructorForSource(sourceID string) (VerifierConstructor, error) {
	constructor, ok := registry[sourceID]
	if !ok {
		return nil, fmt.Errorf("no verifier for sourceID: %s", sourceID)
	}
	return constructor, nil
}
