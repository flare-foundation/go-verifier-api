package btcverifier

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/stretchr/testify/require"

	btcaddr "github.com/flare-foundation/go-flare-common/pkg/btc/address"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"

	apitypes "github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigutxoconfigured/btc/client"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// testMainnetXpub is a fixed BIP-32 mainnet wallet-level (parent) xpub at depth
// 2 (m/87'/0'), the shape ValidateV1 requires. Reused from the btcaddr fixtures.
const testMainnetXpub = "xpub6AA1xY86BDWPPrATRwypWcB7Z5Kxu2fhAdTLDTUXKbL7mMQ9NJXwnvsitFKg3bCMBComzwbdo3Je1zwAY1GMgfrXbtC2gPknszPQETGv2d1"

// utxoXpubs and utxoSignetXpubs are three DISTINCT wallet-level (parent) xpubs
// (deterministic seeds 1..3, m/87'/coin') for genuine k-of-n fixtures — a
// multisig with repeated keys is rejected, so signers must be distinct.
var utxoXpubs = []string{
	"xpub6BT5TaYRGEymDz7PN6eEwJHaZuR3cDbLaLZGAaGn42YLo5rhz76cUcTnJ9zexAewNRCgCG8bV8PmCzveQCnQGfn7sFyWt2P5WnzSo4S7RRa",
	"xpub6AizowepZHn9HVBcyFtePudx2ufnhKNNjQCZLnwuzjBrhXnmwKk7b2SAhcXw1ddfgrme9FyBZUspiMRJmX5bGxQLgoGpLtSTUoeyKY6xTCP",
	"xpub6BbwkRCpSqWb354VTWBzLUEdErvYucHhdDMJ7irvkiXCzR3EJTUH81aKn3JvU9f9ryFCS8JWQr9NNfSF8oB9JyYPB3ERx6DXjfujuUeWLib",
}

var utxoSignetXpubs = []string{
	"tpubDBphypN6yWwDYVt5yPFA4QAeb21XwTLMQAX54ZHquy7ieQbvjLwxqvdR3vbFoy37fmWdT4eXpQbGSpzAHV81gMeBRHGJX4zcb2X5gSwphgQ",
	"tpubDB6dLBUWGZjbc4VwUGoxXtNykPPZKDQaufoZPVpWdNrUbtZzP9UPLi8peEuqPVrGrxPuTbqVqAXYNvfXS91h5t7iPzMBpjwPyf2stD4WpaL",
	"tpubDByaGf2WA7U3LBDuVdERvAqCgRgSP6jFUwJixrHkWLNRarmqX1eWXQL9n4NyDkL5RUB98Bdd6R1a3zNtFQQEAiECPkHpEf89RaYFFhnZjqM",
}

// xpubsBytes maps each xpub string to the base58 wire bytes a request carries.
func xpubsBytes(t *testing.T, xpubs []string) [][]byte {
	t.Helper()
	out := make([][]byte, len(xpubs))
	for i, x := range xpubs {
		out[i] = xpubBytes(t, x)
	}
	return out
}

// mockFetcher is a programmable nodeClient: outs maps "txid:vout" to a UTXO view
// (nil = not found/spent), err forces a gettxout transport failure, chain is the
// chain getblockchaininfo reports (newVerifier defaults it to the params' chain
// so the network pin passes), and chainErr forces a getblockchaininfo failure.
type mockFetcher struct {
	outs     map[string]*client.GetTxOut
	err      error
	chain    string
	chainErr error
}

func (m *mockFetcher) GetTxOut(_ context.Context, txid string, vout uint32, _ bool) (*client.GetTxOut, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.outs[fmt.Sprintf("%s:%d", txid, vout)], nil
}

func (m *mockFetcher) Chain(context.Context) (string, error) {
	if m.chainErr != nil {
		return "", m.chainErr
	}
	return m.chain, nil
}

// xpubBytes is the wire shape a request carries for a public key: the base58
// xpub STRING the chain registers, as its bytes (the verifier is base58-only).
// The decode is a sanity check that the fixture xpub is well-formed.
func xpubBytes(t *testing.T, xpub string) []byte {
	t.Helper()
	require.Len(t, base58.Decode(xpub), serializedExtendedKeyLen+4)
	return []byte(xpub)
}

func txid(b byte) [32]byte {
	var t [32]byte
	for i := range t {
		t[i] = b
	}
	return t
}

// validRequest builds a 2-of-3 request with N anchors at distinct outpoints.
func validRequest(t *testing.T, n int) fdc2.IPMWMultisigUtxoConfiguredRequestBody {
	t.Helper()
	anchors := make([]fdc2.IPMWMultisigUtxoConfiguredAnchor, n)
	for i := range n {
		anchors[i] = fdc2.IPMWMultisigUtxoConfiguredAnchor{
			GenesisAnchorTxid: txid(byte(i + 1)),
			GenesisAnchorVout: uint32(i),
		}
	}
	return fdc2.IPMWMultisigUtxoConfiguredRequestBody{
		AccountIndex: 0,
		PublicKeys:   xpubsBytes(t, utxoXpubs),
		Threshold:    2,
		Anchors:      anchors,
	}
}

// expectedOuts derives the per-anchor scriptPubKey hex for req and returns a
// map programmed with valid UTXOs (value at the floor) for every anchor, plus
// the chain-0 address.
func expectedOuts(t *testing.T, req fdc2.IPMWMultisigUtxoConfiguredRequestBody, params *chaincfg.Params) (map[string]*client.GetTxOut, string) {
	t.Helper()
	bac, err := toBtcAccountConfigured(req)
	require.NoError(t, err)
	accXpubs, err := btcaddr.DeriveAccountXpubs(bac.Xpubs, bac.AccountIndex, params)
	require.NoError(t, err)

	outs := make(map[string]*client.GetTxOut, len(req.Anchors))
	var chain0 string
	for i, a := range req.Anchors {
		addr, _, _, err := btcaddr.Derive(accXpubs, bac.Threshold, btcaddr.External, uint32(i), params)
		require.NoError(t, err)
		if i == 0 {
			chain0 = addr.EncodeAddress()
		}
		spk, err := txscript.PayToAddrScript(addr)
		require.NoError(t, err)
		key := fmt.Sprintf("%s:%d", hex.EncodeToString(a.GenesisAnchorTxid[:]), a.GenesisAnchorVout)
		outs[key] = &client.GetTxOut{
			Confirmations: 6,
			Value:         "0.00010000", // exactly MinAnchorValueSat
			ScriptPubKey:  client.ScriptPubKey{Hex: hex.EncodeToString(spk), Type: "witness_v0_scripthash"},
		}
	}
	return outs, chain0
}

func newVerifier(fetcher nodeClient, params *chaincfg.Params) *BtcVerifier {
	// Default the mock's reported chain to the one the params expect so the
	// network pin passes transparently; tests exercising the pin set it directly.
	if m, ok := fetcher.(*mockFetcher); ok && m.chain == "" && m.chainErr == nil {
		m.chain, _ = expectedChain(params)
	}
	return &BtcVerifier{
		Config: &config.PMWMultisigUtxoConfig{
			EncodedAndABI: config.EncodedAndABI{
				SourceIDPair: config.SourceIDEncodedPair{SourceID: config.SourceBTC},
			},
		},
		Client: fetcher,
		Params: params,
	}
}

func TestVerifyAllAnchorsValid(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 3)
	outs, chain0 := expectedOuts(t, req, params)

	v := newVerifier(&mockFetcher{outs: outs}, params)
	res, err := v.Verify(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint8(apitypes.PMWMultisigUtxoStatusOK), res.Status)
	require.Equal(t, chain0, res.AccountAddress)
}

func TestVerifyAnchorMissingIsError(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 2)
	outs, _ := expectedOuts(t, req, params)
	// Drop the second anchor's UTXO — simulates not found / spent.
	delete(outs, fmt.Sprintf("%s:%d", hex.EncodeToString(req.Anchors[1].GenesisAnchorTxid[:]), req.Anchors[1].GenesisAnchorVout))

	v := newVerifier(&mockFetcher{outs: outs}, params)
	res, err := v.Verify(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint8(apitypes.PMWMultisigUtxoStatusERROR), res.Status)
	require.Empty(t, res.AccountAddress)
}

func TestVerifySubValueIsError(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 1)
	outs, _ := expectedOuts(t, req, params)
	for k := range outs {
		outs[k].Value = "0.00009999" // one sat below the floor
	}

	v := newVerifier(&mockFetcher{outs: outs}, params)
	res, err := v.Verify(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint8(apitypes.PMWMultisigUtxoStatusERROR), res.Status)
}

func TestVerifyScriptMismatchIsError(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 1)
	outs, _ := expectedOuts(t, req, params)
	for k := range outs {
		outs[k].ScriptPubKey.Hex = "0020" + hex.EncodeToString(make([]byte, 32)) // wrong program
	}

	v := newVerifier(&mockFetcher{outs: outs}, params)
	res, err := v.Verify(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint8(apitypes.PMWMultisigUtxoStatusERROR), res.Status)
}

func TestVerifyRPCErrorPropagates(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 1)

	rpcErr := errors.New("connection refused")
	v := newVerifier(&mockFetcher{err: rpcErr}, params)
	res, err := v.Verify(context.Background(), req)
	require.Error(t, err)
	require.ErrorIs(t, err, rpcErr)
	// Must not masquerade as an ERROR status.
	require.Equal(t, uint8(0), res.Status)
	require.Empty(t, res.AccountAddress)
}

func TestVerifyMalformedRequests(t *testing.T) {
	params := &chaincfg.MainNetParams
	pk := xpubBytes(t, testMainnetXpub)
	base := func() fdc2.IPMWMultisigUtxoConfiguredRequestBody { return validRequest(t, 2) }

	tests := []struct {
		name   string
		mutate func(*fdc2.IPMWMultisigUtxoConfiguredRequestBody)
	}{
		{"thresholdZero", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) { r.Threshold = 0 }},
		{"thresholdAboveN", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) { r.Threshold = 99 }},
		{"accountIndexHardened", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) { r.AccountIndex = 1 << 31 }},
		{"duplicateAnchors", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) { r.Anchors[1] = r.Anchors[0] }},
		{"emptyAnchors", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) { r.Anchors = nil }},
		{"badXpubLen", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) { r.PublicKeys = [][]byte{pk[:10], pk, pk} }},
		{"duplicateKeys", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) {
			// Repeat a signer: collapses k-of-n independence, must be rejected.
			r.PublicKeys = [][]byte{r.PublicKeys[0], r.PublicKeys[0], r.PublicKeys[1]}
		}},
		{"tooManyKeys", func(r *fdc2.IPMWMultisigUtxoConfiguredRequestBody) {
			keys := make([][]byte, 21)
			for i := range keys {
				keys[i] = pk
			}
			r.PublicKeys = keys
		}},
	}
	// A fetcher that would accept anything, so a non-rejection would surface as OK.
	fetcher := &mockFetcher{outs: map[string]*client.GetTxOut{}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := base()
			tc.mutate(&req)
			v := newVerifier(fetcher, params)
			_, err := v.Verify(context.Background(), req)
			require.ErrorIs(t, err, ErrInvalidRequest)
		})
	}
}

// TestXpubStringFromBytesBase58Only pins the base58-only behaviour: the base58
// xpub string the chain registers round-trips, while inputs that are not a
// base58 xpub string are refused.
func TestXpubStringFromBytesBase58Only(t *testing.T) {
	b := xpubBytes(t, testMainnetXpub) // the base58 string bytes
	got, err := xpubStringFromBytes(b)
	require.NoError(t, err)
	require.Equal(t, testMainnetXpub, got)

	// A truncated string is not a valid xpub.
	_, err = xpubStringFromBytes(b[:20])
	require.Error(t, err)

	// A raw byte blob that is not a base58 xpub string is refused.
	raw := base58.Decode(testMainnetXpub)[:serializedExtendedKeyLen]
	_, err = xpubStringFromBytes(raw)
	require.Error(t, err)
}

func TestUnsupportedSourceParams(t *testing.T) {
	_, err := paramsForSource("nope")
	require.ErrorIs(t, err, ErrUnsupportedSource)
}

// TestResolveNetworkParams covers the explicit-override / source-default split:
// BTC_NETWORK wins when set (including regtest, which no source id names), an
// unknown value fails, and an empty value falls back to the source default.
func TestResolveNetworkParams(t *testing.T) {
	p, err := resolveNetworkParams("regtest", config.SourceTestBTC)
	require.NoError(t, err)
	require.Equal(t, &chaincfg.RegressionNetParams, p)

	// Override wins over the source-implied default.
	p, err = resolveNetworkParams("Mainnet", config.SourceTestBTC)
	require.NoError(t, err)
	require.Equal(t, &chaincfg.MainNetParams, p)

	_, err = resolveNetworkParams("nope", config.SourceBTC)
	require.ErrorIs(t, err, ErrUnsupportedNetwork)

	// Empty override falls back to the source default: BTC → mainnet,
	// testBTC → signet.
	p, err = resolveNetworkParams("", config.SourceBTC)
	require.NoError(t, err)
	require.Equal(t, &chaincfg.MainNetParams, p)

	p, err = resolveNetworkParams("", config.SourceTestBTC)
	require.NoError(t, err)
	require.Equal(t, &chaincfg.SigNetParams, p)
}

// TestNewBtcVerifierUnsupportedSource confirms the constructor rejects a config
// whose source id has no Bitcoin network mapping.
func TestNewBtcVerifierUnsupportedSource(t *testing.T) {
	cfg := &config.PMWMultisigUtxoConfig{
		EncodedAndABI: config.EncodedAndABI{
			SourceIDPair: config.SourceIDEncodedPair{SourceID: "nope"},
		},
		SourceRPCURL: "http://127.0.0.1:1",
	}
	_, err := NewBtcVerifier(cfg)
	require.ErrorIs(t, err, ErrUnsupportedSource)
}

// TestVerifyAnchorMalformedValueIsError confirms a UTXO whose value string is
// not a valid decimal amount surfaces as an error (not a false ERROR/OK) — the
// value can't be parsed, so the anchor cannot be judged.
func TestVerifyAnchorMalformedValueIsError(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 1)
	outs, _ := expectedOuts(t, req, params)
	for k := range outs {
		outs[k].Value = "not-a-number"
	}
	v := newVerifier(&mockFetcher{outs: outs}, params)
	_, err := v.Verify(context.Background(), req)
	require.Error(t, err)
}

// TestVerifyUnconfirmedAnchorIsError proves an anchor confirmed fewer than
// minAnchorConfirmations deep (a shallow/reorg-risk or fabricated output) is
// rejected as ERROR rather than accepted — defense-in-depth beyond
// includeMempool=false, and the reorg-safety buffer. Uses the boundary
// (minAnchorConfirmations-1) so it stays correct if the depth changes.
func TestVerifyUnconfirmedAnchorIsError(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 1)
	outs, _ := expectedOuts(t, req, params)
	for k := range outs {
		outs[k].Confirmations = minAnchorConfirmations - 1
	}
	v := newVerifier(&mockFetcher{outs: outs}, params)
	res, err := v.Verify(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint8(apitypes.PMWMultisigUtxoStatusERROR), res.Status)
	require.Empty(t, res.AccountAddress)
}

// TestVerifyAnchorIndexBinding proves anchors[i] is matched against the address
// derived at chain-index i specifically: funding anchor[1]'s outpoint with the
// UTXO that belongs at anchor[0]'s address must NOT verify. This catches an
// off-by-one or a lost positional binding that the tautological happy-path test
// (which derives every expected address at the same index the code uses) cannot.
func TestVerifyAnchorIndexBinding(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 2)
	outs, _ := expectedOuts(t, req, params)

	key := func(a fdc2.IPMWMultisigUtxoConfiguredAnchor) string {
		return fmt.Sprintf("%s:%d", hex.EncodeToString(a.GenesisAnchorTxid[:]), a.GenesisAnchorVout)
	}
	// Give anchor[1]'s outpoint the UTXO (scriptPubKey) derived for chain 0.
	// Its true chain-1 address differs, so the script match must fail -> ERROR.
	outs[key(req.Anchors[1])] = outs[key(req.Anchors[0])]

	v := newVerifier(&mockFetcher{outs: outs}, params)
	res, err := v.Verify(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint8(apitypes.PMWMultisigUtxoStatusERROR), res.Status)
	require.Empty(t, res.AccountAddress)
}

// TestVerifyTestBTCSignetAddress exercises the testBTC -> signet network path
// end-to-end: a signet (tpub) parent xpub must derive tb1 anchor addresses and
// verify against them. Every other happy-path test uses mainnet, so this is the
// only coverage that a wrong network parameter would surface.
func TestVerifyTestBTCSignetAddress(t *testing.T) {
	params := &chaincfg.SigNetParams
	req := fdc2.IPMWMultisigUtxoConfiguredRequestBody{
		AccountIndex: 0,
		PublicKeys:   xpubsBytes(t, utxoSignetXpubs),
		Threshold:    2,
		Anchors:      []fdc2.IPMWMultisigUtxoConfiguredAnchor{{GenesisAnchorTxid: txid(1), GenesisAnchorVout: 0}},
	}
	outs, chain0 := expectedOuts(t, req, params)
	require.True(t, strings.HasPrefix(chain0, "tb1"), "signet address must be tb1..., got %q", chain0)

	v := newVerifier(&mockFetcher{outs: outs}, params)
	res, err := v.Verify(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint8(apitypes.PMWMultisigUtxoStatusOK), res.Status)
	require.Equal(t, chain0, res.AccountAddress)
	require.True(t, strings.HasPrefix(res.AccountAddress, "tb1"))
}

func TestExpectedChain(t *testing.T) {
	cases := []struct {
		params *chaincfg.Params
		want   string
		ok     bool
	}{
		{&chaincfg.MainNetParams, "main", true},
		{&chaincfg.TestNet3Params, "test", true},
		{&chaincfg.SigNetParams, "signet", true},
		{&chaincfg.RegressionNetParams, "regtest", true},
		{&chaincfg.Params{Net: 0}, "", false},
	}
	for _, c := range cases {
		got, ok := expectedChain(c.params)
		require.Equal(t, c.ok, ok)
		require.Equal(t, c.want, got)
	}
}

// TestVerifyRejectsWrongChainNode: a node serving a different chain than the
// verifier expects fails closed with ErrNetworkMismatch before any anchor lookup,
// so a cheaply funded wrong-chain UTXO can never satisfy an anchor.
func TestVerifyRejectsWrongChainNode(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 3)
	outs, _ := expectedOuts(t, req, params)

	v := newVerifier(&mockFetcher{outs: outs, chain: "test"}, params)
	res, err := v.Verify(context.Background(), req)
	require.ErrorIs(t, err, ErrNetworkMismatch)
	require.Equal(t, uint8(0), res.Status)
	require.Empty(t, res.AccountAddress)
}

// TestVerifyUnreachableNodeFailsClosed: when the chain cannot be read, the
// request path stays fail-closed — Verify returns the wrapped fetch error, never
// a status.
func TestVerifyUnreachableNodeFailsClosed(t *testing.T) {
	params := &chaincfg.MainNetParams
	req := validRequest(t, 1)
	outs, _ := expectedOuts(t, req, params)

	probeErr := fmt.Errorf("%w: dial tcp: connection refused", client.ErrFetchChainInfo)
	v := newVerifier(&mockFetcher{outs: outs, chainErr: probeErr}, params)
	res, err := v.Verify(context.Background(), req)
	require.ErrorIs(t, err, client.ErrFetchChainInfo)
	require.Equal(t, uint8(0), res.Status)
}

// TestVerifyNetworkStartup: a confirmed wrong chain fails boot; an unreachable
// node does not block boot (the request path stays fail-closed separately).
func TestVerifyNetworkStartup(t *testing.T) {
	params := &chaincfg.MainNetParams

	wrong := newVerifier(&mockFetcher{chain: "signet"}, params)
	require.ErrorIs(t, wrong.VerifyNetwork(context.Background()), ErrNetworkMismatch)

	unreachable := newVerifier(&mockFetcher{chainErr: client.ErrFetchChainInfo}, params)
	require.NoError(t, unreachable.VerifyNetwork(context.Background()))

	ok := newVerifier(&mockFetcher{chain: "main"}, params)
	require.NoError(t, ok.VerifyNetwork(context.Background()))
}

// TestNetworkVerifyTTL: a fresh confirmation is trusted (no re-probe) until the
// TTL lapses, after which a node repointed to a wrong chain is re-detected.
func TestNetworkVerifyTTL(t *testing.T) {
	params := &chaincfg.MainNetParams
	fetcher := &mockFetcher{chain: "main"}
	now := time.Unix(1_700_000_000, 0)
	v := &BtcVerifier{Client: fetcher, Params: params, now: func() time.Time { return now }}

	require.NoError(t, v.ensureNetworkVerified(context.Background()))

	// Node repointed to a wrong chain, but within the TTL the cached pass holds.
	fetcher.chain = "test"
	now = now.Add(networkVerifyTTL - time.Minute)
	require.NoError(t, v.ensureNetworkVerified(context.Background()))

	// Past the TTL the wrong chain is re-detected.
	now = now.Add(2 * time.Minute)
	require.ErrorIs(t, v.ensureNetworkVerified(context.Background()), ErrNetworkMismatch)
}

// TestCheckNetworkUnmappedParamsSkips: with params that have no chain mapping the
// pin is skipped (marked verified), so unusual deployments are not blocked.
func TestCheckNetworkUnmappedParamsSkips(t *testing.T) {
	v := &BtcVerifier{
		Client: &mockFetcher{chainErr: client.ErrFetchChainInfo},
		Params: &chaincfg.Params{Net: 0},
	}
	require.NoError(t, v.checkNetwork(context.Background()))
	require.True(t, v.verifiedFresh())
}
