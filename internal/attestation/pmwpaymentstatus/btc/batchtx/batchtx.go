// Package batchtx holds the shape of a located PMW batch transaction, shared by
// the two sources that can produce one: a Bitcoin node addressed by txid
// (package nodechain, the CSP path) and a verifier-utxo-indexer database
// (package indexerdb, the TeePaymentsUtxo path).
//
// The type lives on its own so neither source has to import the other, and so
// the node path does not depend on a package named for a database it never
// reads.
package batchtx

import (
	"fmt"
	"math/big"
	"strings"
)

// satPerBTC is the number of satoshis in one BTC. Bitcoin reports amounts as
// decimal BTC, converted to satoshis here.
const satPerBTC = 100_000_000

// MaxMoneySat is the total Bitcoin supply in satoshis (21,000,000 BTC). No single
// output or transaction can hold more, so a larger value is corrupt or forged data
// from the semi-trusted node — rejected rather than summed (fail closed, and it
// keeps output/input sums well inside int64 so they cannot overflow).
const MaxMoneySat int64 = 21_000_000 * satPerBTC

// maxBTCAmountLen bounds the decimal string a node/indexer may present for an
// amount. A real amount is at most 8 integer digits + '.' + 8 fractional digits
// (17 chars); the slack tolerates leading/trailing zeros. Anything longer is
// rejected before big.Rat parses it, so a hostile RPC response with millions of
// digits cannot drive unbounded parsing work.
const maxBTCAmountLen = 32

// Output is one transaction output: its locking script and value in satoshis.
type Output struct {
	PkScript []byte
	Value    int64
}

// BatchTx is a located PMW batch transaction and the fields the verifier needs.
type BatchTx struct {
	Txid           string
	BlockNumber    uint64
	BlockTimestamp uint64
	// Fee is the batch transaction's own fee in satoshis (sum of input values
	// minus sum of output values). It excludes any CPFP fee-bump child, which is
	// a separate transaction.
	Fee     int64
	Outputs []Output // in output (n) order

	// AnchorInputAddress is the address paid by the output that input[0] spends.
	//
	// It is what binds a transaction to an anchor chain: every batch on chain i
	// spends, and recreates, an output paying that chain's anchor address. A
	// source that cannot report it leaves it empty, and a caller that needs the
	// binding must then refuse rather than assume.
	AnchorInputAddress string

	// Confirmations is the depth of the containing block on the ACTIVE chain.
	// Zero or negative means the block is not on the active chain — which a
	// txid lookup alone does not rule out, because a node keeps serving
	// transactions whose block was reorged away.
	Confirmations int64
}

// SatsFromBTC converts a decimal BTC amount string (the node/indexer
// representation) to satoshis exactly, failing on sub-satoshi precision,
// overflow, or malformed input. It uses exact rational arithmetic so no amount
// is rounded through a float.
func SatsFromBTC(v string) (int64, error) {
	// Bound the input length BEFORE big.Rat parses it: a real amount is short, and
	// a hostile node could otherwise send millions of digits to burn CPU/memory.
	// Report only the length, never echo the oversized string into logs.
	if len(v) > maxBTCAmountLen {
		return 0, fmt.Errorf("BTC amount too long: %d characters (max %d)", len(v), maxBTCAmountLen)
	}
	if v == "" || strings.ContainsAny(v, "/eE") {
		return 0, fmt.Errorf("invalid BTC amount %q", v)
	}
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		return 0, fmt.Errorf("invalid BTC amount %q", v)
	}
	if r.Sign() < 0 {
		return 0, fmt.Errorf("negative BTC amount %q", v)
	}
	r.Mul(r, new(big.Rat).SetInt64(satPerBTC))
	if !r.IsInt() {
		return 0, fmt.Errorf("BTC amount %q has sub-satoshi precision", v)
	}
	n := r.Num()
	if !n.IsInt64() {
		return 0, fmt.Errorf("BTC amount %q overflows int64 satoshis", v)
	}
	sats := n.Int64()
	if sats > MaxMoneySat {
		return 0, fmt.Errorf("BTC amount %q exceeds the total money supply", v)
	}
	return sats, nil
}
