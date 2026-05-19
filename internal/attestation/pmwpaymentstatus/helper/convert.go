package helper

import (
	"fmt"
	"math/big"
)

// ParseNonNegativeBigInt parses a base-10 decimal string into a non-negative *big.Int.
//
// Use only for fields that are unsigned by construction in the indexer
// JSON payloads consumed by this package:
//   - transaction fees (XRPL drops, always ≥ 0 per protocol)
//   - native XRP balances (AccountRoot.Balance, enforced ≥ 0 at the ledger)
//
// Do not use for IOU/issued-currency balances (RippleState.Balance) or IOU
// amount objects — those are signed by protocol design (they can represent
// debt).
//
// A negative value in an unsigned context indicates corrupted or malicious
// indexer data and is rejected here so downstream code never sees one.
func ParseNonNegativeBigInt(s string) (*big.Int, error) {
	const decimalBase = 10
	i, ok := new(big.Int).SetString(s, decimalBase)
	if !ok {
		return nil, fmt.Errorf("invalid big.Int string: %s", s)
	}
	if i.Sign() < 0 {
		return nil, fmt.Errorf("expected non-negative big.Int, got %s", s)
	}
	return i, nil
}
