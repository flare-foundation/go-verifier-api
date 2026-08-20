package btcverifier

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"

	btcaddr "github.com/flare-foundation/go-flare-common/pkg/btc/address"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"

	apitypes "github.com/flare-foundation/go-verifier-api/internal/api/types"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwmultisigutxoconfigured/btc/client"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

var (
	// ErrInvalidRequest is returned when the request shape violates documented
	// constraints (bad xpub, out-of-range threshold/accountIndex, malformed
	// anchor set). Maps to HTTP 400.
	ErrInvalidRequest = errors.New("invalid utxo multisig request")
	// ErrUnsupportedSource is returned when the configured source id has no
	// Bitcoin network mapping.
	ErrUnsupportedSource = errors.New("unsupported source id for BTC verifier")
	// ErrUnsupportedNetwork is returned when BTC_NETWORK names a network with no
	// chaincfg mapping.
	ErrUnsupportedNetwork = errors.New("unsupported BTC_NETWORK value")
	// ErrNetworkMismatch marks a Bitcoin node confirmed to be serving a different
	// chain than the verifier expects (a wrong-chain node). Maps to HTTP 503.
	ErrNetworkMismatch = errors.New("bitcoin node is on the wrong network")
)

// serializedExtendedKeyLen is the byte length of a BIP-32 serialized extended
// key (version 4, depth 1, parent fingerprint 4, child number 4, chain code 32,
// key data 33).
const serializedExtendedKeyLen = 78

// minAnchorConfirmations is the minimum confirmation depth an anchor UTXO must
// have to be accepted. Beyond rejecting unconfirmed/fabricated outputs (gettxout
// is already called with includeMempool=false), it provides a reorg-safety
// buffer: an anchor confirmed only 1–2 blocks deep at registration could be
// reorged out, leaving the account bound on-chain to a vanished UTXO. Six blocks
// is Bitcoin's conventional finality; four is a pragmatic buffer that keeps
// registration latency reasonable while making a reorg that unwinds an anchor
// unlikely. Must be kept consistent across verifiers so they cannot disagree on
// a borderline anchor (the relay client enforces its own ReorgSafetyDepth for
// ongoing operation, separately from this registration-time check).
const minAnchorConfirmations uint64 = 4

// coinbaseMaturity is the number of confirmations a coinbase output needs before
// it is spendable under Bitcoin's consensus rules. A coinbase anchor below it
// cannot be spent yet (and a coinbase is discarded wholesale if its block is
// reorged out), so binding an account to an immature coinbase UTXO is unsafe —
// such anchors are rejected until they reach maturity.
const coinbaseMaturity uint64 = 100

// nodeClient abstracts the Bitcoin node lookups so the verifier can be unit
// tested without a live node. *client.Client satisfies it.
type nodeClient interface {
	GetTxOut(ctx context.Context, txid string, vout uint32, includeMempool bool) (*client.GetTxOut, error)
	Chain(ctx context.Context) (string, error)
}

// BtcVerifier verifies a PMWMultisigUtxoConfigured (BtcAccountConfigured)
// attestation against a Bitcoin node: it validates the request shape, derives
// each per-chain P2WSH anchor address, and confirms every anchor UTXO exists,
// is unspent, meets the value floor, and pays the derived address.
type BtcVerifier struct {
	Config *config.PMWMultisigUtxoConfig
	Client nodeClient
	Params *chaincfg.Params

	// lastVerifiedNano is the unix-nano time the node's chain was last confirmed
	// to match Params (0 = never); verifyMu serializes confirmation attempts.
	// Until it is fresh (within networkVerifyTTL) Verify fails closed, so a node
	// repointed to a different chain is re-detected within the TTL.
	lastVerifiedNano atomic.Int64
	verifyMu         sync.Mutex
	now              func() time.Time // overridable in tests
}

func NewBtcVerifier(cfg *config.PMWMultisigUtxoConfig) (*BtcVerifier, error) {
	params, err := resolveNetworkParams(cfg.BtcNetwork, cfg.SourceIDPair.SourceID)
	if err != nil {
		return nil, err
	}
	return &BtcVerifier{
		Config: cfg,
		Client: client.NewClient(cfg.SourceRPCURL),
		Params: params,
		now:    time.Now,
	}, nil
}

const (
	// networkPinTimeout bounds a single getblockchaininfo probe used to pin the chain.
	networkPinTimeout = 5 * time.Second
	// networkVerifyTTL is how long a confirmed chain is trusted before Verify
	// re-checks, so a node repointed to a different chain is caught within it.
	networkVerifyTTL = 30 * time.Minute
)

// clock returns the current time, using the injected now when set (tests).
func (v *BtcVerifier) clock() time.Time {
	if v.now != nil {
		return v.now()
	}
	return time.Now()
}

// verifiedFresh reports whether the chain was confirmed within networkVerifyTTL.
func (v *BtcVerifier) verifiedFresh() bool {
	last := v.lastVerifiedNano.Load()
	return last != 0 && v.clock().Sub(time.Unix(0, last)) < networkVerifyTTL
}

// expectedChain returns the getblockchaininfo chain name Params corresponds to
// ("main", "test", "signet" or "regtest"), and whether the params are mapped.
func expectedChain(params *chaincfg.Params) (string, bool) {
	switch params.Net {
	case chaincfg.MainNetParams.Net:
		return "main", true
	case chaincfg.TestNet3Params.Net:
		return "test", true
	case chaincfg.SigNetParams.Net:
		return "signet", true
	case chaincfg.RegressionNetParams.Net:
		return "regtest", true
	default:
		return "", false
	}
}

// checkNetwork probes the node's chain once and classifies the result: nil (and
// marks the verifier verified) when it matches Params; ErrNetworkMismatch when it
// is a confirmed wrong chain; a wrapped fetch error when the chain cannot be read
// (unreachable node).
func (v *BtcVerifier) checkNetwork(ctx context.Context) error {
	expected, ok := expectedChain(v.Params)
	if !ok {
		v.lastVerifiedNano.Store(v.clock().UnixNano())
		return nil
	}
	got, err := v.Client.Chain(ctx)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("%w: node chain %q but verifier expects %q",
			ErrNetworkMismatch, got, expected)
	}
	v.lastVerifiedNano.Store(v.clock().UnixNano())
	return nil
}

// VerifyNetwork pins the configured Bitcoin node to the chain the verifier's
// network parameters expect, so a node misconfigured to a different chain — whose
// address encodings may coincide (signet/testnet/regtest all share the "tb"/"bcrt"
// prefixes and tpub version) — cannot let a cheaply funded wrong-chain UTXO
// satisfy an anchor. Run once at startup: a confirmed wrong chain fails boot,
// while an unreachable node does not block boot — the request path stays
// fail-closed via ensureNetworkVerified until the chain is confirmed.
func (v *BtcVerifier) VerifyNetwork(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, networkPinTimeout)
	defer cancel()

	err := v.checkNetwork(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNetworkMismatch):
		return err
	default:
		logger.Warnf("PMWMultisigUtxoConfigured: Bitcoin chain not verified at startup for source %s: %v; requests are blocked until it verifies",
			v.Config.SourceIDPair.SourceID, err)
		return nil
	}
}

// ensureNetworkVerified fails closed until the node's chain has been confirmed. A
// fresh confirmation (within networkVerifyTTL) is a lock-free no-op; otherwise it
// re-probes — so a wrong-chain or unreachable node keeps every request rejected,
// and a node repointed to a different chain is caught within the TTL.
func (v *BtcVerifier) ensureNetworkVerified(ctx context.Context) error {
	if v.verifiedFresh() {
		return nil
	}
	v.verifyMu.Lock()
	defer v.verifyMu.Unlock()
	if v.verifiedFresh() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, networkPinTimeout)
	defer cancel()
	return v.checkNetwork(ctx)
}

// networkParamsByName maps an explicit BTC_NETWORK value to its Bitcoin network
// parameters. regtest and testnet are reachable only through this override — no
// source id names them — which is what lets the KAT/e2e harness run against a
// local regtest node.
var networkParamsByName = map[string]*chaincfg.Params{
	"mainnet":  &chaincfg.MainNetParams,
	"signet":   &chaincfg.SigNetParams,
	"testnet":  &chaincfg.TestNet3Params,
	"testnet3": &chaincfg.TestNet3Params,
	"regtest":  &chaincfg.RegressionNetParams,
}

// resolveNetworkParams picks the verifier's Bitcoin network parameters. An
// explicit BTC_NETWORK value wins so a deployment can target regtest/testnet,
// which no source id names; when unset the source id implies the default
// (paramsForSource). The chain (source id) and the network (BTC_NETWORK) are
// deliberately separable: a mainnet-assuming verifier pointed at signet/regtest
// would derive addresses that match nothing and answer "not found" silently, so
// an unknown override fails construction rather than defaulting.
func resolveNetworkParams(network string, source config.SourceName) (*chaincfg.Params, error) {
	if network != "" {
		params, ok := networkParamsByName[strings.ToLower(network)]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedNetwork, network)
		}
		return params, nil
	}
	return paramsForSource(source)
}

// paramsForSource maps the configured source id to its default Bitcoin network
// parameters when BTC_NETWORK is unset. testBTC uses signet, which shares the
// "tb" Bech32 HRP and the tpub extended-key version with testnet3, so derived
// addresses match either test network the node may run.
func paramsForSource(source config.SourceName) (*chaincfg.Params, error) {
	switch source {
	case config.SourceBTC:
		return &chaincfg.MainNetParams, nil
	case config.SourceTestBTC:
		return &chaincfg.SigNetParams, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSource, source)
	}
}

func (v *BtcVerifier) Verify(ctx context.Context, req fdc2.IPMWMultisigUtxoConfiguredRequestBody) (fdc2.IPMWMultisigUtxoConfiguredResponseBody, error) {
	// Enforce the request shape here so direct ABI callers cannot bypass the
	// bounds applied by the JSON ToInternal path. ValidateV1 checks the account
	// index range, the k-of-n bounds (n <= 20), xpub parsing/network/depth, and
	// the anchor set (1 <= N <= 32, distinct outpoints).
	bac, err := toBtcAccountConfigured(req)
	if err != nil {
		return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	if err := bac.ValidateV1(v.Params); err != nil {
		return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}

	accXpubs, err := btcaddr.DeriveAccountXpubs(bac.Xpubs, bac.AccountIndex, v.Params)
	if err != nil {
		return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}

	// Fail closed until the node's chain is confirmed to match the verifier's
	// network: a node that could not be verified at startup (unreachable then, or
	// a wrong chain) must not answer anchor lookups against the wrong chain.
	if err := v.ensureNetworkVerified(ctx); err != nil {
		return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, err
	}

	// Each anchors[i] is chain i, whose anchor UTXO must live at the P2WSH
	// multisig derived at external-chain leaf i. Chain 0's address is the
	// account address echoed back on success.
	//
	// The per-request node-lookup fan-out (one gettxout per anchor) is already
	// bounded: ValidateV1 -> ValidateAnchorSet caps the set at MaxAnchors (32),
	// so a request can force at most 32 RPCs — no unbounded amplification. The
	// primary mitigation is economic: each request is a paid Flare transaction
	// whose governance-set attestation fee is set to exceed evaluation cost, so
	// spam pays for itself rather than taxing the DP set for free.
	accountAddress := ""
	for i, anchor := range bac.Anchors {
		addr, witnessScript, _, err := btcaddr.Derive(accXpubs, bac.Threshold, btcaddr.External, uint32(i), v.Params)
		if err != nil {
			return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, fmt.Errorf("%w: deriving anchor %d address: %w", ErrInvalidRequest, i, err)
		}
		if i == 0 {
			accountAddress = addr.EncodeAddress()
		}
		expectedScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, fmt.Errorf("%w: building script for anchor %d: %w", ErrInvalidRequest, i, err)
		}

		// Txid is display (big-endian) order, which is what gettxout expects.
		txid := hex.EncodeToString(anchor.Txid[:])
		// includeMempool=false is BY DESIGN, not a conservative default — do NOT
		// flip it. The check must read only *confirmed* chain state, because that
		// is the sole view every honest verifier shares. Mempool contents differ
		// per node (a spend one verifier sees, another does not), so reading them
		// would make two honest verifiers disagree on a borderline anchor and
		// break the threshold agreement the whole attestation rests on. The
		// residual it leaves — an anchor spent only in the mempool still reads as
		// unspent here — is not an attacker vector: genesis anchors are the
		// wallet's OWN outpoints, so a pre-registration mempool spend is operator
		// self-harm that fails downstream anyway. Confirmed-only is both the
		// agreement-preserving and the more robust lens for a binding we pin.
		utxo, err := v.Client.GetTxOut(ctx, txid, anchor.Vout, false)
		if err != nil {
			// Transport/RPC failure — surface as an error so the caller retries,
			// never a false ERROR status.
			return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, err
		}
		ok, err := anchorValid(utxo, expectedScript, witnessScript)
		if err != nil {
			return fdc2.IPMWMultisigUtxoConfiguredResponseBody{}, err
		}
		if !ok {
			return errorResponse(), nil
		}
	}

	return fdc2.IPMWMultisigUtxoConfiguredResponseBody{
		Status:         uint8(apitypes.PMWMultisigUtxoStatusOK),
		AccountAddress: accountAddress,
	}, nil
}

// anchorValid reports whether utxo is a confirmed, unspent output that meets the
// anchor value floor and pays the expected P2WSH script. The witnessScript is
// unused for the on-chain match (the derived scriptPubKey already encodes its
// hash) but kept in the signature to document the P2WSH relationship. A nil utxo
// (output not found or spent) is invalid, not an error.
func anchorValid(utxo *client.GetTxOut, expectedScript, _ []byte) (bool, error) {
	if utxo == nil {
		return false, nil
	}
	if utxo.Confirmations < minAnchorConfirmations {
		return false, nil
	}
	// A coinbase output is unspendable until it reaches coinbaseMaturity, so an
	// immature coinbase anchor cannot back an account (and reorgs discard the
	// whole coinbase); reject it until it matures.
	if utxo.Coinbase && utxo.Confirmations < coinbaseMaturity {
		return false, nil
	}
	valueSat, err := utxo.ValueSat()
	if err != nil {
		return false, fmt.Errorf("parsing anchor value: %w", err)
	}
	if valueSat < btcaddr.MinAnchorValueSat {
		return false, nil
	}
	return utxo.ScriptPubKey.Hex == hex.EncodeToString(expectedScript), nil
}

func errorResponse() fdc2.IPMWMultisigUtxoConfiguredResponseBody {
	return fdc2.IPMWMultisigUtxoConfiguredResponseBody{
		Status:         uint8(apitypes.PMWMultisigUtxoStatusERROR),
		AccountAddress: "",
	}
}

// toBtcAccountConfigured adapts the ABI request body to the btcaddr reference
// type. The wire carries each public key as its base58 xpub string, which is
// handed to hdkeychain for parsing.
func toBtcAccountConfigured(req fdc2.IPMWMultisigUtxoConfiguredRequestBody) (btcaddr.BtcAccountConfigured, error) {
	xpubs := make([]string, len(req.PublicKeys))
	for i, pk := range req.PublicKeys {
		xpub, err := xpubStringFromBytes(pk)
		if err != nil {
			return btcaddr.BtcAccountConfigured{}, fmt.Errorf("publicKeys[%d]: %w", i, err)
		}
		xpubs[i] = xpub
	}

	anchors := make([]btcaddr.AnchorBinding, len(req.Anchors))
	for i, a := range req.Anchors {
		anchors[i] = btcaddr.AnchorBinding{Txid: a.GenesisAnchorTxid, Vout: a.GenesisAnchorVout}
	}

	// Guard the uint64->int narrowing: a threshold beyond int range would wrap on
	// a 32-bit platform, potentially to a small positive value that slips past
	// ValidateV1's k-of-n bound. Legitimate thresholds are tiny (<= n <= 20), so
	// reject anything that cannot fit int on every platform before the cast.
	if req.Threshold > math.MaxInt32 {
		return btcaddr.BtcAccountConfigured{}, fmt.Errorf("threshold %d out of range", req.Threshold)
	}

	return btcaddr.BtcAccountConfigured{
		AccountIndex: req.AccountIndex,
		Xpubs:        xpubs,
		Threshold:    int(req.Threshold),
		Anchors:      anchors,
	}, nil
}

// isXpubString reports whether the bytes are a base58check-encoded extended key:
// an ~111-character string that base58-decodes to the serialized-key payload
// plus its 4-byte checksum.
func isXpubString(s string) bool {
	if len(s) < 100 || len(s) > 120 {
		return false
	}
	return len(base58.Decode(s)) == serializedExtendedKeyLen+4
}

// xpubStringFromBytes interprets a request public key as the base58check xpub
// string the chain registers — the canonical form TeePaymentsConfigVerifier
// ._checkWalletPublicKeys compares byte-for-byte against the wallet's keys. The
// string's checksum, network and depth are validated downstream by
// btcaddr.DeriveAccountXpubs.
func xpubStringFromBytes(b []byte) (string, error) {
	s := string(b)
	if !isXpubString(s) {
		return "", fmt.Errorf("expected a base58 xpub string, got %d bytes", len(b))
	}
	return s, nil
}
