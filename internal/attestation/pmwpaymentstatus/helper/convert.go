package helper

import (
	"fmt"
	"math/big"
)

// MaxXRPDrops is the total XRP supply expressed in drops (100e9 XRP × 1e6
// drops/XRP = 1e17). No legitimate XRPL amount — fee, balance, or fee budget —
// can exceed it, so a larger value is physically impossible and signals corrupt
// indexer data. It is an immutable uint64 constant; use ExceedsMaxXRPDrops to
// test a decoded value against it.
const MaxXRPDrops uint64 = 100_000_000_000 * 1_000_000

// ExceedsMaxXRPDrops reports whether v (a non-negative drops amount) is above
// the total XRP supply — i.e. physically impossible and therefore corrupt data.
// A value that does not fit in uint64 necessarily exceeds the supply. A nil
// value is treated as exceeding (fail closed) rather than panicking, so callers
// need not nil-check before calling.
func ExceedsMaxXRPDrops(v *big.Int) bool {
	if v == nil {
		return true
	}
	return !v.IsUint64() || v.Uint64() > MaxXRPDrops
}

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
// maxDropsDigits caps the length of a decimal drops string before it is parsed,
// so a maliciously long value from a corrupt indexer cannot drive quadratic-time
// big.Int parsing (a CPU-exhaustion vector). The total XRP supply is 1e17 drops
// (18 digits); 25 is generous headroom. Values that fit the length but still
// exceed the supply are caught downstream by ExceedsMaxXRPDrops.
const maxDropsDigits = 25

func ParseNonNegativeBigInt(s string) (*big.Int, error) {
	if len(s) > maxDropsDigits {
		return nil, fmt.Errorf("decimal value too long: %d chars (max %d)", len(s), maxDropsDigits)
	}
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
