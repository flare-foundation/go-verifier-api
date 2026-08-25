package btcverifier

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	btcbatch "github.com/flare-foundation/go-flare-common/pkg/btc/batch"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/payments"
	"gorm.io/gorm"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc/batchtx"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/btc/nodechain"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	teeinstruction "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/instruction"
	"github.com/flare-foundation/go-verifier-api/internal/config"
)

// ErrUnsupportedSource is returned when the configured source id has no Bitcoin
// network mapping.
var ErrUnsupportedSource = errors.New("unsupported source id for BTC payment-status verifier")

// ErrNetworkMismatch marks a Bitcoin node confirmed to be serving a different
// chain than the verifier expects (a wrong-chain node). Maps to HTTP 503.
var ErrNetworkMismatch = errors.New("bitcoin node is on the wrong network")

// ErrNetworkUnverified is the fail-closed error returned when the node's chain
// has not yet been confirmed and a probe is in flight or recently failed with no
// cached cause. Maps to HTTP 503.
var ErrNetworkUnverified = errors.New("bitcoin chain not yet verified")

const (
	// networkPinTimeout bounds a single getblockchaininfo probe used to pin the chain.
	networkPinTimeout = 5 * time.Second
	// networkVerifyTTL is how long a confirmed chain is trusted before Verify
	// re-checks, so a node repointed to a different chain is caught within it.
	networkVerifyTTL = 30 * time.Minute
	// networkProbeCooldown bounds how often an unverified node is re-probed while
	// it stays unreachable/wrong, so a request burst does not stampede probes.
	networkProbeCooldown = 10 * time.Second
)

// probeResult boxes a probe's error so it can be stored atomically. A nil err
// means the last probe succeeded.
type probeResult struct{ err error }

// chainProber reports the chain a Bitcoin node serves; *nodechain.Repo satisfies
// it. An interface so the pin can be exercised without a live node.
type chainProber interface {
	Chain(ctx context.Context) (string, error)
}

// logRepo is the C-chain log lookup the verifier needs; *paymentdb.DBRepo
// satisfies it. An interface so tests can substitute a stub.
type logRepo interface {
	FetchInstructionLogsForID(ctx context.Context, eventHash string, instructionID common.Hash) ([]*types.Log, error)
	FetchLogsByInstructionTopic1(ctx context.Context, eventHash string, instructionID common.Hash) ([]*types.Log, error)
}

// ErrMissingTransactionID is returned when a CSP request omits the settling
// txid. It is a malformed request, not a payment that did not happen: without a
// locator there is nothing to check, and answering "not found" would let an
// omission read as evidence.
var ErrMissingTransactionID = errors.New("the request carries no settling transaction id")

// locator is everything the sources need to find one settling batch. Each uses
// the part it can: a node is addressed by txid, an index by the position.
type locator struct {
	// TransactionID is the request's locator, in display byte order.
	TransactionID string
	// AnchorAddress is the address every batch on this anchor chain reuses.
	AnchorAddress string
	Nonce         uint64
}

// batchSource locates the settling batch transaction for a payment.
//
// Two implementations, and the difference between them is the whole point of
// this type: nodeSource asks a Bitcoin node for ONE TRANSACTION BY ID, which
// -txindex=1 answers with no index and no backfill; dbSource SEARCHES a
// verifier-utxo-indexer for the batch at a position, which needs the indexer's
// pmw_anchor_address / pmw_nonce columns populated over history.
//
// Neither is trusted to have found the right transaction. The verifier checks
// the result against the on-chain instruction either way.
type batchSource interface {
	// AnchorAddress resolves the address paid by the output at (txid, vout),
	// used to turn a chain's genesis anchor outpoint into the anchor address
	// its batches reuse. Returns "" when the output cannot be resolved.
	AnchorAddress(ctx context.Context, txid string, vout uint32) (string, error)
	// Batch returns the settling batch, or nil when it is not confirmed on the
	// active chain.
	Batch(ctx context.Context, loc locator) (*batchtx.BatchTx, error)
}

// nodeSource reads the settling batch straight from a Bitcoin node by txid.
type nodeSource struct{ repo *nodechain.Repo }

func (s nodeSource) AnchorAddress(ctx context.Context, txid string, vout uint32) (string, error) {
	return s.repo.OutputAddress(ctx, txid, vout)
}

func (s nodeSource) Batch(ctx context.Context, loc locator) (*batchtx.BatchTx, error) {
	if loc.TransactionID == "" {
		return nil, ErrMissingTransactionID
	}
	return s.repo.Batch(ctx, loc.TransactionID)
}

// messageSource supplies the per-payment instruction messages of one batch.
//
// TWO dispatch paths produce them, and they are not the same events:
//
//   - TeePaymentsUtxo emits one TeeInstructionsSent per payment, from the TEE
//     diamond. Each is an instruction a machine acts on.
//   - The CSP channel emits one PaymentBatched per payment at settle, from the
//     channel. Those are attestation RECORDS, not instructions: CSP dispatches
//     a single batch instruction to the machines and no machine acts on a
//     per-payment message.
//
// The attestation itself is identical either way — a payment is proven against
// the batch that carried it — so the difference is confined here rather than
// spread through Verify.
type messageSource interface {
	Messages(ctx context.Context, instructionID, opType common.Hash) (
		[]*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, error)
}

// BtcVerifier verifies a PMWPaymentStatus attestation for a Bitcoin payment: it
// resolves the payment to its batch, recomputes the batch instruction id, reads
// the UtxoPaymentInstructionMessage from the C-chain, locates the settling batch
// transaction in the verifier-utxo-indexer database, and matches the payment's
// output group against the instruction. It is the BTC source verifier for the
// shared PMWPaymentStatus attestation type (the XRP sibling lives in ../xrp).
type BtcVerifier struct {
	Repo     logRepo
	Messages messageSource
	Source   batchSource
	Resolver Resolver
	Config   *config.PMWPaymentStatusConfig
	Params   *chaincfg.Params
	// prober reads the node's chain for the network pin (VerifyNetwork /
	// ensureNetworkVerified); nil disables the pin (tests that inject stub sources).
	prober chainProber

	// Request-path pin state, mirroring PMWMultisigUtxoConfigured: lastVerifiedNano
	// is when the chain was last confirmed (0 = never); until it is fresh (within
	// networkVerifyTTL) Verify fails closed, so a node repointed to another chain
	// is re-detected within the TTL rather than minting false not-founds.
	lastVerifiedNano atomic.Int64
	verifyMu         sync.Mutex
	lastAttemptNano  atomic.Int64
	lastProbeErr     atomic.Pointer[probeResult]
	now              func() time.Time // overridable in tests
}

// clock returns the current time, using the injected now when set (tests).
func (v *BtcVerifier) clock() time.Time {
	if v.now != nil {
		return v.now()
	}
	return time.Now()
}

// verifiedFresh reports whether the chain was confirmed within networkVerifyTTL.
// The age >= 0 bound guards a wall-clock rollback from extending trust.
func (v *BtcVerifier) verifiedFresh() bool {
	last := v.lastVerifiedNano.Load()
	if last == 0 {
		return false
	}
	age := v.clock().Sub(time.Unix(0, last))
	return age >= 0 && age < networkVerifyTTL
}

// checkNetwork probes the node's chain once: nil (and marks verified) when it
// matches Params; ErrNetworkMismatch on a confirmed wrong chain; a wrapped fetch
// error when the chain cannot be read.
func (v *BtcVerifier) checkNetwork(ctx context.Context) error {
	expected, ok := expectedChain(v.Params)
	if !ok {
		return fmt.Errorf("%w: params have no chain mapping", ErrUnsupportedSource)
	}
	got, err := v.prober.Chain(ctx)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("%w: node chain %q but verifier expects %q", ErrNetworkMismatch, got, expected)
	}
	v.lastVerifiedNano.Store(v.clock().UnixNano())
	return nil
}

// ensureNetworkVerified fails closed until the node's chain has been confirmed. A
// fresh confirmation is a lock-free no-op; otherwise it re-probes without
// stampeding (TryLock + a per-cooldown negative cache), so a wrong-chain or
// unreachable node keeps every request rejected and a live repoint is caught
// within the TTL. A nil prober disables the pin (tests with stub sources).
func (v *BtcVerifier) ensureNetworkVerified(ctx context.Context) error {
	if v.prober == nil || v.verifiedFresh() {
		return nil
	}
	if !v.verifyMu.TryLock() {
		return v.cachedProbeErr()
	}
	defer v.verifyMu.Unlock()
	if v.verifiedFresh() {
		return nil
	}
	if last := v.lastAttemptNano.Load(); last != 0 {
		if age := v.clock().Sub(time.Unix(0, last)); age >= 0 && age < networkProbeCooldown {
			return v.cachedProbeErr()
		}
	}
	ctx, cancel := context.WithTimeout(ctx, networkPinTimeout)
	defer cancel()
	err := v.checkNetwork(ctx)
	v.lastAttemptNano.Store(v.clock().UnixNano())
	v.lastProbeErr.Store(&probeResult{err: err})
	return err
}

// cachedProbeErr returns the most recent probe's error so a stampeding caller can
// fail closed without launching its own probe.
func (v *BtcVerifier) cachedProbeErr() error {
	if p := v.lastProbeErr.Load(); p != nil && p.err != nil {
		return p.err
	}
	return ErrNetworkUnverified
}

// NewBtcVerifier wires a BtcVerifier from config, the verifier-utxo-indexer
// database (sourceDB), and the C-chain database. It dials the Flare RPC (for
// getBatchPaymentId / getAnchor) and resolves the source's Bitcoin network.
// Mirrors the XRP verifier's (sourceDB, cChainDB) shape.
func NewBtcVerifier(cfg *config.PMWPaymentStatusConfig, cChainDB *gorm.DB) (*BtcVerifier, error) {
	params, err := paramsForSource(cfg.SourceIDPair.SourceID, cfg.BtcNetwork)
	if err != nil {
		return nil, err
	}
	// This landing supports the node path only: per-payment records come from the
	// channel's PaymentBatched events and the settling batch is read from the
	// Bitcoin node by the txid the request carries. The verifier-utxo-indexer
	// (diamond) path is deferred, so a deployment without CHANNEL_ADDRESS fails
	// closed at construction rather than booting a half-configured verifier.
	if cfg.ChannelAddress == (common.Address{}) {
		return nil, fmt.Errorf("%w: CHANNEL_ADDRESS is required (the indexer path is not yet supported)", ErrUnsupportedSource)
	}
	resolver, err := NewOnChainResolver(cfg.FlareRPCURL, cfg.TeePaymentsContractAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot create on-chain resolver: %w", err)
	}
	repo := paymentdb.NewDBRepo(nil, cChainDB, cfg.ChannelAddress)
	channelABI, err := abi.JSON(strings.NewReader(paymentBatchedABI))
	if err != nil {
		return nil, fmt.Errorf("cannot parse the PaymentBatched ABI: %w", err)
	}
	nodeRepo := nodechain.NewRepo(cfg.SourceRPCURL, settlementMinConfirmations)
	return &BtcVerifier{
		Repo:     repo,
		Messages: PaymentBatchedMessages{Repo: repo, ABI: channelABI},
		Source:   nodeSource{repo: nodeRepo},
		Resolver: resolver,
		Config:   cfg,
		Params:   params,
		prober:   nodeRepo,
		now:      time.Now,
	}, nil
}

// settlementMinConfirmations is the confirmation-depth floor a settling batch
// must meet. It is a FIXED constant, not per-deployment config: the depth changes
// the verdict (status 0 vs 2 for a borderline-depth batch), so every data
// provider must use the same value or they split attestation consensus. Six
// blocks is Bitcoin's conventional finality for a proof that closes a redemption.
const settlementMinConfirmations uint64 = 6

// expectedChain returns the getblockchaininfo chain name the verifier's params
// correspond to ("main", "test", "signet" or "regtest"), and whether they map.
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

// VerifyNetwork pins the configured Bitcoin node to the chain the verifier's
// network expects. Run once at startup: a confirmed wrong chain fails boot; an
// unreachable node does not block boot, because the request path stays fail-closed
// via ensureNetworkVerified (which re-checks within networkVerifyTTL and rejects
// until the chain is confirmed). Mirrors the PMWMultisigUtxoConfigured pin.
func (v *BtcVerifier) VerifyNetwork(ctx context.Context) error {
	if v.prober == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, networkPinTimeout)
	defer cancel()
	err := v.checkNetwork(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNetworkMismatch):
		return err
	default:
		logger.Warnf("PMWPaymentStatus: Bitcoin chain not verified at startup for source %s: %v; requests fail closed until it verifies",
			v.Config.SourceIDPair.SourceID, err)
		return nil
	}
}

// Close releases the resolver's RPC connection, mirroring the XRP verifier.
func (v *BtcVerifier) Close() error {
	if c, ok := v.Resolver.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// paramsForSource resolves the network, keeping this package's sentinel so
// callers can switch on it.
func paramsForSource(source config.SourceName, network string) (*chaincfg.Params, error) {
	params, err := config.BtcNetworkParams(source, network)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedSource, err)
	}
	return params, nil
}

func (v *BtcVerifier) Verify(ctx context.Context, req fdc2.IPMWPaymentStatusRequestBody) (fdc2.IPMWPaymentStatusResponseBody, error) {
	empty := fdc2.IPMWPaymentStatusResponseBody{}

	// Fail closed until the node's chain is confirmed. Defense-in-depth alongside
	// the fail-closed anchor lookup: a node repointed to a different chain after
	// boot is re-detected within networkVerifyTTL rather than answering off it.
	if err := v.ensureNetworkVerified(ctx); err != nil {
		return empty, err
	}

	account := req.SenderAddress
	sourceID := v.Config.SourceIDPair.SourceIDEncoded

	// A batch's payments share one instruction id, derived from the batch's first
	// payment id. Resolve it on-chain (the BTC analog of the XRP initialNonce
	// binding); a never-issued paymentId fails closed here.
	batchPaymentID, err := v.Resolver.ResolveBatch(ctx, sourceID, account, req.PaymentId)
	if err != nil {
		return empty, err
	}
	instructionID, err := teeinstruction.GenerateInstructionID(req.OpType, sourceID, account, batchPaymentID)
	if err != nil {
		return empty, fmt.Errorf("cannot generate instruction ID: %w", err)
	}
	// One per-payment message per payment, all sharing the batch instruction id;
	// select the one carrying the requested paymentId.
	msgs, err := v.Messages.Messages(ctx, instructionID, req.OpType)
	if err != nil {
		return empty, err
	}
	message, err := selectPayment(msgs, req)
	if err != nil {
		return empty, err
	}
	if err := checkMessageConsistency(message, sourceID, account, req.PaymentId, batchPaymentID); err != nil {
		return empty, err
	}
	// This verifier proves only native satoshi delivery: it matches value outputs
	// to the recipient script, nothing about a token. A non-empty TokenId means the
	// instruction is for another asset, so attesting delivery from the sat outputs
	// would sign a claim about an asset never checked. Reject it (as the XRP path
	// does), fail closed rather than mis-prove.
	if len(message.TokenId) != 0 {
		return empty, fmt.Errorf("non-native BTC payment (TokenId set) is not supported: %w", paymentdb.ErrDatabase)
	}

	// Locate the settling batch. The anchor chain is identified by its reused
	// anchor address: resolve the chain's genesis anchor outpoint on-chain and
	// turn it into an address. The anchor is registry-guaranteed to exist, so —
	// unlike an absent settling batch below — an unresolvable anchor is NOT a
	// legitimate not-found: it means the node cannot see a transaction the chain
	// definitely has (no -txindex, unsynced, or wrong chain). Fail closed so a
	// misconfigured node cannot mint a false status-2.
	anchorTxid, anchorVout, err := v.Resolver.ResolveAnchorOutpoint(ctx, sourceID, account, message.AnchorIndex)
	if err != nil {
		return empty, err
	}
	anchorAddress, err := v.Source.AnchorAddress(ctx, anchorTxid, anchorVout)
	if err != nil {
		return empty, err
	}
	if anchorAddress == "" {
		return empty, fmt.Errorf("registry anchor %s:%d for chain %d did not resolve to an address: %w",
			anchorTxid, anchorVout, message.AnchorIndex, paymentdb.ErrDatabase)
	}
	batch, err := v.Source.Batch(ctx, locator{
		TransactionID: txidToHex(req.TransactionId),
		AnchorAddress: anchorAddress,
		Nonce:         message.Nonce,
	})
	if err != nil {
		return empty, err
	}
	if batch == nil {
		return notFoundResponse(message), nil
	}

	// Bind the transaction to its position. The request's txid is a LOCATOR, so
	// nothing so far establishes that this transaction is the one that settled
	// this payment — these two checks do, and together they leave no room for a
	// wrong one to pass.
	//
	// The nonce is re-derived from the outputs rather than taken from any
	// precomputed column, and it must be the instruction's.
	parsed, err := btcbatch.ParseBatch(toBatchOutputs(batch.Outputs))
	if err != nil {
		return empty, fmt.Errorf("batch %s is not a PMW batch: %v: %w", batch.Txid, err, paymentdb.ErrDatabase)
	}
	if parsed.Nonce != message.Nonce {
		return empty, fmt.Errorf("batch %s nonce %d != instruction nonce %d: %w",
			batch.Txid, parsed.Nonce, message.Nonce, paymentdb.ErrDatabase)
	}
	// And input[0] must spend this chain's anchor. Only k-of-n can do that, so
	// an outsider cannot forge a transaction that reaches here: the nonce
	// OP_RETURN is free to write, but the anchor is not free to spend. This is the
	// forgery guard, so it is mandatory: the node path always resolves input[0]'s
	// prevout, and an empty value means the transaction did not spend a resolvable
	// address as input[0] — fail closed rather than skip the binding.
	if batch.AnchorInputAddress != anchorAddress {
		return empty, fmt.Errorf(
			"batch %s spends %q as input[0], not anchor chain %d's address %s: %w",
			batch.Txid, batch.AnchorInputAddress, message.AnchorIndex, anchorAddress, paymentdb.ErrDatabase)
	}

	// This payment's group is at offset (paymentId - batchPaymentId): a batch's
	// ids are the contiguous range [batchPaymentId, batchPaymentId + paymentCount).
	groupIdx := req.PaymentId - batchPaymentID
	if groupIdx >= uint64(len(parsed.Groups)) {
		return empty, fmt.Errorf("payment %d (group %d) out of range for batch %s with %d groups: %w", req.PaymentId, groupIdx, batch.Txid, len(parsed.Groups), paymentdb.ErrDatabase)
	}

	recipientScript, err := v.recipientScript(message.RecipientAddress)
	if err != nil {
		return empty, err
	}
	amount, err := satoshis(message.Amount)
	if err != nil {
		return empty, err
	}
	status, received, err := btcbatch.Match(parsed.Groups[groupIdx], btcbatch.Expected{
		Reference: message.PaymentReference[:],
		Recipient: recipientScript,
		Amount:    amount,
	})
	if err != nil {
		return empty, fmt.Errorf("payment %d contradicts batch %s: %v: %w", req.PaymentId, batch.Txid, err, paymentdb.ErrDatabase)
	}
	// btcbatch.Match reports StatusUndeliverable purely from "zero value delivered
	// to the recipient" — it does NOT check dust (by design; see its doc). So an
	// omitted or censored payment reads the same as a legitimately sub-dust one.
	// Confirm the claim: an amount at or above the deterministic dust threshold
	// could have been delivered, so a zero delivery for it is an inconsistency
	// (fail closed), not a settled sub_dust status. The threshold uses Bitcoin
	// Core's fixed DUST_RELAY_TX_FEE so every verifier computes the same value.
	if status == btcbatch.StatusUndeliverable && amount >= dustThresholdSat(recipientScript) {
		return empty, fmt.Errorf(
			"payment %d delivered nothing but its amount %d >= dust threshold %d for the recipient: %w",
			req.PaymentId, amount, dustThresholdSat(recipientScript), paymentdb.ErrDatabase)
	}
	return buildResponse(message, status, received, batch)
}

// dustRelayFeeSatPerKvB is Bitcoin Core's default DUST_RELAY_TX_FEE; the IsDust
// threshold scales linearly with it. Fixed (not a live feerate) so every verifier
// computes the same threshold — a disagreement would refuse honest batches or
// admit the sweep this guards against.
const dustRelayFeeSatPerKvB int64 = 3000

// dustThresholdSat is Bitcoin Core's IsDust threshold for an output paying
// pkScript: dustRelayFee * nSize / 1000, where nSize is the serialized output
// size plus the assumed cost of spending it (67 vbytes witness, 148 legacy). At
// the 3000 sat/kvB default: P2WPKH 294, P2WSH 330, P2TR 330, P2PKH 546. Mirrors
// the proposer's and PMWUtxoProposalCheck's dust check so the two agree.
func dustThresholdSat(pkScript []byte) int64 {
	outputSize := int64(8 + 1 + len(pkScript))
	const witnessScale = 4
	if isWitnessProgram(pkScript) {
		return (outputSize + (32 + 4 + 1 + 107/witnessScale + 4)) * dustRelayFeeSatPerKvB / 1000
	}
	return (outputSize + (32 + 4 + 1 + 107 + 4)) * dustRelayFeeSatPerKvB / 1000
}

// isWitnessProgram reports whether pkScript is a SegWit witness program: a
// version opcode (OP_0, or OP_1..OP_16) followed by a single 2..40 byte push.
func isWitnessProgram(pkScript []byte) bool {
	if len(pkScript) < 4 || len(pkScript) > 42 {
		return false
	}
	if pkScript[0] != 0x00 && (pkScript[0] < 0x51 || pkScript[0] > 0x60) {
		return false
	}
	pushLen := int(pkScript[1])
	return pushLen >= 2 && pushLen <= 40 && len(pkScript) == 2+pushLen
}

// selectPayment decodes every batch event and returns the message whose
// paymentId matches the request. Zero matches is a not-found (the payment was
// never instructed).
//
// More than one is NORMAL, not an inconsistency: a REISSUED batch is dispatched
// again, so every payment in it is re-emitted once per attempt. Each attempt
// respends the same anchor outpoint at the same nonce, which is what makes them
// mutually exclusive on Bitcoin — so the repeats describe the same payment and
// resolve to the same answer, whichever one is used.
//
// What is still an inconsistency is repeats that DISAGREE about the payment.
// The tee/key pairs legitimately differ between attempts (a reissue may be
// assigned a different machine set), so they are not compared; everything the
// answer depends on is.
func selectPayment(msgs []*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, req fdc2.IPMWPaymentStatusRequestBody) (*payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, error) {
	var found *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage
	for _, m := range msgs {
		if m.PaymentId != req.PaymentId {
			continue
		}
		if found != nil && !sameInstruction(found, m) {
			return nil, fmt.Errorf("conflicting messages for paymentId %d: %w", req.PaymentId, paymentdb.ErrDatabase)
		}
		found = m
	}
	if found == nil {
		return nil, fmt.Errorf("no message for paymentId %d in batch: %w", req.PaymentId, paymentdb.ErrRecordNotFound)
	}
	return found, nil
}

// sameInstruction reports whether two emissions describe the same payment in
// the same batch — every field this verifier's answer is derived from.
func sameInstruction(a, b *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage) bool {
	return a.WalletId == b.WalletId &&
		a.SourceId == b.SourceId &&
		a.AccountAddress == b.AccountAddress &&
		a.AccountIndex == b.AccountIndex &&
		a.AnchorIndex == b.AnchorIndex &&
		a.RecipientAddress == b.RecipientAddress &&
		bytes.Equal(a.TokenId, b.TokenId) &&
		equalBig(a.Amount, b.Amount) &&
		equalBig(a.MaxFee, b.MaxFee) &&
		a.PaymentReference == b.PaymentReference &&
		a.Nonce == b.Nonce &&
		a.PaymentId == b.PaymentId &&
		a.BatchPaymentId == b.BatchPaymentId
}

func equalBig(a, b *big.Int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Cmp(b) == 0
}

// checkMessageConsistency binds the decoded message to the request and resolved
// batch. Any mismatch is a C-chain index inconsistency (fail closed).
func checkMessageConsistency(m *payments.ITeePaymentsUtxoUtxoPaymentInstructionMessage, sourceID common.Hash, account string, paymentID, batchPaymentID uint64) error {
	if common.Hash(m.SourceId) != sourceID {
		return fmt.Errorf("event sourceId %s != expected %s: %w", common.Hash(m.SourceId).Hex(), sourceID.Hex(), paymentdb.ErrDatabase)
	}
	if m.AccountAddress != account {
		return fmt.Errorf("event accountAddress %q != expected %q: %w", m.AccountAddress, account, paymentdb.ErrDatabase)
	}
	if m.PaymentId != paymentID {
		return fmt.Errorf("event paymentId %d != expected %d: %w", m.PaymentId, paymentID, paymentdb.ErrDatabase)
	}
	if m.BatchPaymentId != batchPaymentID {
		return fmt.Errorf("event batchPaymentId %d != resolved %d: %w", m.BatchPaymentId, batchPaymentID, paymentdb.ErrDatabase)
	}
	return nil
}

// recipientScript decodes the instruction's recipient address to its
// scriptPubKey. The address is validated on-chain before the instruction is
// emitted, so a decode failure here is a C-chain inconsistency (fail closed).
func (v *BtcVerifier) recipientScript(addr string) ([]byte, error) {
	a, err := btcutil.DecodeAddress(addr, v.Params)
	if err != nil {
		return nil, fmt.Errorf("instruction recipient address %q is invalid: %v: %w", addr, err, paymentdb.ErrDatabase)
	}
	if !a.IsForNet(v.Params) {
		return nil, fmt.Errorf("instruction recipient address %q is not for the configured network: %w", addr, paymentdb.ErrDatabase)
	}
	script, err := txscript.PayToAddrScript(a)
	if err != nil {
		return nil, fmt.Errorf("cannot build script for recipient %q: %v: %w", addr, err, paymentdb.ErrDatabase)
	}
	return script, nil
}

// satoshis converts an instruction amount to int64 satoshis, failing closed on a
// nil, negative, or impossibly large value (max BTC supply < MaxInt64).
func satoshis(v *big.Int) (int64, error) {
	if v == nil || v.Sign() < 0 || !v.IsInt64() {
		return 0, fmt.Errorf("instruction amount %v is out of satoshi range: %w", v, paymentdb.ErrDatabase)
	}
	return v.Int64(), nil
}

// toBatchOutputs projects the indexer's outputs into the batch parser's neutral
// Output form (identical fields; a distinct type keeps the DB layer decoupled
// from the shared grammar package).
// txidToHex renders the request's settling transaction id as the hex string a
// node expects, in display byte order — the same order txidToBytes32 decodes.
// A zero value yields "", which sources read as "no locator supplied".
func txidToHex(id [32]byte) string {
	if id == ([32]byte{}) {
		return ""
	}
	return hex.EncodeToString(id[:])
}

func toBatchOutputs(outs []batchtx.Output) []btcbatch.Output {
	converted := make([]btcbatch.Output, len(outs))
	for i, o := range outs {
		converted[i] = btcbatch.Output{PkScript: o.PkScript, Value: o.Value}
	}
	return converted
}
