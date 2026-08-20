package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// jsonRPCRequest is the Bitcoin Core JSON-RPC request envelope.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

// jsonRPCError is the Bitcoin Core JSON-RPC error object.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// getTxOutResponse is the JSON-RPC envelope for a gettxout call. Result is nil
// when the output does not exist or is already spent (Bitcoin Core returns a
// null result in that case).
type getTxOutResponse struct {
	Result *GetTxOut     `json:"result"`
	Error  *jsonRPCError `json:"error"`
}

// getBlockchainInfoResponse is the JSON-RPC envelope for getblockchaininfo; only
// the chain field is read, to pin the node to the network the verifier expects.
type getBlockchainInfoResponse struct {
	Result *BlockchainInfo `json:"result"`
	Error  *jsonRPCError   `json:"error"`
}

// BlockchainInfo is the subset of getblockchaininfo the verifier reads: the
// node's chain, one of "main", "test", "signet" or "regtest".
type BlockchainInfo struct {
	Chain string `json:"chain"`
}

// ScriptPubKey is the locking script of a transaction output.
type ScriptPubKey struct {
	Hex     string `json:"hex"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

// GetTxOut is the unspent-output view returned by Bitcoin Core's gettxout.
// Value is the output amount in BTC as a decimal number; use ValueSat to obtain
// the exact satoshi value without float rounding.
type GetTxOut struct {
	Confirmations uint64       `json:"confirmations"`
	Value         json.Number  `json:"value"`
	ScriptPubKey  ScriptPubKey `json:"scriptPubKey"`
	Coinbase      bool         `json:"coinbase"`
}

// ValueSat converts the BTC-denominated Value to satoshis exactly, parsing the
// decimal string rather than the float so no precision is lost. Bitcoin Core
// always emits amounts with at most 8 fractional digits.
func (o *GetTxOut) ValueSat() (int64, error) {
	return btcToSat(o.Value.String())
}

// maxMoneySat is the total Bitcoin supply in satoshis (21,000,000 BTC). No
// output can hold more, so a larger value is physically impossible and signals
// corrupt or forged data from the semi-trusted node — it is rejected rather
// than parsed (fail closed, mirroring the XRP side's MaxXRPDrops bound).
const maxMoneySat int64 = 21_000_000 * 100_000_000

// btcToSat parses a decimal BTC amount string (e.g. "0.00010000") into satoshis.
// It rejects negative amounts, anything with more than 8 fractional digits, and
// any value exceeding the total money supply (which also fails closed on the
// int64 overflow an attacker-supplied oversized string would otherwise cause).
func btcToSat(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty BTC amount")
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("negative BTC amount %q", s)
	}
	intPart, fracPart, _ := strings.Cut(s, ".")
	if len(fracPart) > 8 {
		return 0, fmt.Errorf("BTC amount %q has more than 8 fractional digits", s)
	}
	// Right-pad the fractional part to 8 digits (satoshi precision).
	fracPart += strings.Repeat("0", 8-len(fracPart))
	digits := intPart + fracPart
	var sat int64
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("BTC amount %q contains a non-digit", s)
		}
		// Bound each step so an oversized/forged value fails closed instead of
		// overflowing int64. sat stays <= maxMoneySat, so sat*10 is safe.
		sat = sat*10 + int64(c-'0')
		if sat > maxMoneySat {
			return 0, fmt.Errorf("BTC amount %q exceeds the maximum money supply", s)
		}
	}
	return sat, nil
}
