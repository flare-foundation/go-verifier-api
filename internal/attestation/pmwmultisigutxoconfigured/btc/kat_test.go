package btcverifier

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	btcaddr "github.com/flare-foundation/go-flare-common/pkg/btc/address"
	"github.com/stretchr/testify/require"
)

// TestKnownAnswerP2WSH is a repo-level known-answer test: for a fixed
// (xpubs, threshold, accountIndex, anchorIndex) it pins the exact P2WSH address
// and scriptPubKey the verifier derives. The expected values are cross-checked
// against an INDEPENDENT reconstruction (raw witness-script assembly below, not
// btcaddr.Derive), so a derivation regression in the imported library — or in
// our accountIndex/chain/leaf wiring — is caught here, not just internal
// consistency. Inputs: utxoXpubs (three distinct mainnet parent xpubs, seeds
// 1..3 at m/87'/0'), 2-of-3, accountIndex 0, chain 0 (external), leaf/anchor 0.
func TestKnownAnswerP2WSH(t *testing.T) {
	params := &chaincfg.MainNetParams
	const (
		threshold    = 2
		accountIndex = 0
		anchorIndex  = 0
		// Pinned known answers (also verified by independentP2WSH below).
		wantAddress = "bc1qfpg88srvztmwc90ysgpsky4fdam76rr3sa79wd5hr48780d3jn9skmjtdh"
		wantScript  = "0020485073c06c12f6ec15e482030b12a96f77ed0c71877c5736971d4fe3bdb194cb"
	)

	// Production path — exactly what BtcVerifier.Verify uses (the imported
	// go-flare-common btcaddr).
	accXpubs, err := btcaddr.DeriveAccountXpubs(utxoXpubs, accountIndex, params)
	require.NoError(t, err)
	addr, _, _, err := btcaddr.Derive(accXpubs, threshold, btcaddr.External, anchorIndex, params)
	require.NoError(t, err)
	gotAddr := addr.EncodeAddress()
	spk, err := txscript.PayToAddrScript(addr)
	require.NoError(t, err)
	gotScript := hex.EncodeToString(spk)

	// Independent reconstruction (no btcaddr): derive each parent xpub down
	// account->chain->leaf, BIP-67 sort, hand-assemble OP_k <pk...> OP_n
	// OP_CHECKMULTISIG, SHA256, bech32 witness-v0.
	wantAddrIndep, wantScriptIndep := independentP2WSH(t, utxoXpubs, threshold, accountIndex, int(btcaddr.External), anchorIndex, params)
	require.Equal(t, wantAddrIndep, gotAddr, "production address must match independent reconstruction")
	require.Equal(t, wantScriptIndep, gotScript, "production scriptPubKey must match independent reconstruction")

	// Pin the literals so a future change to either path is caught.
	require.Equal(t, wantAddress, gotAddr, "update the pinned address only after confirming the change is intended")
	require.Equal(t, wantScript, gotScript)
}

// independentP2WSH reconstructs the sorted k-of-n P2WSH address and scriptPubKey
// from parent xpubs without using btcaddr: CKDpub account->chain->leaf per key,
// BIP-67 sort, raw-byte OP_k <pk...> OP_n OP_CHECKMULTISIG, SHA256, witness-v0.
func independentP2WSH(t *testing.T, parentXpubs []string, k, accountIndex, chain int, leaf uint32, params *chaincfg.Params) (addr, scriptHex string) {
	t.Helper()
	pubs := make([][]byte, len(parentXpubs))
	for i, xs := range parentXpubs {
		key, err := hdkeychain.NewKeyFromString(xs)
		require.NoError(t, err)
		acct, err := key.Derive(uint32(accountIndex))
		require.NoError(t, err)
		ck, err := acct.Derive(uint32(chain))
		require.NoError(t, err)
		lk, err := ck.Derive(leaf)
		require.NoError(t, err)
		pk, err := lk.ECPubKey()
		require.NoError(t, err)
		pubs[i] = pk.SerializeCompressed()
	}
	sort.Slice(pubs, func(i, j int) bool { return bytes.Compare(pubs[i], pubs[j]) < 0 })

	n := len(pubs)
	require.True(t, k >= 1 && k <= 16 && n >= 1 && n <= 16, "helper supports 1..16")
	var script []byte
	script = append(script, byte(txscript.OP_1-1+k)) // OP_k
	for _, pk := range pubs {
		script = append(script, byte(len(pk)))
		script = append(script, pk...)
	}
	script = append(script, byte(txscript.OP_1-1+n)) // OP_n
	script = append(script, txscript.OP_CHECKMULTISIG)

	h := sha256.Sum256(script)
	a, err := btcutil.NewAddressWitnessScriptHash(h[:], params)
	require.NoError(t, err)
	pkScript, err := txscript.PayToAddrScript(a)
	require.NoError(t, err)
	return a.EncodeAddress(), hex.EncodeToString(pkScript)
}
