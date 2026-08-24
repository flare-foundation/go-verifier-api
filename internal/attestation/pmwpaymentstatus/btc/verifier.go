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

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	btcbatch "github.com/flare-foundation/go-flare-common/pkg/btc/batch"
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
	return &BtcVerifier{
		Repo:     repo,
		Messages: PaymentBatchedMessages{Repo: repo, ABI: channelABI},
		Source:   nodeSource{repo: nodechain.NewRepo(cfg.SourceRPCURL, cfg.MinConfirmations)},
		Resolver: resolver,
		Config:   cfg,
		Params:   params,
	}, nil
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

	// Locate the settling batch. The anchor chain is identified by its reused
	// anchor address: resolve the chain's genesis anchor outpoint on-chain and
	// turn it into an address. An absent anchor or batch means the settlement is
	// not confirmed — a not-found (status 2) attestation.
	anchorTxid, anchorVout, err := v.Resolver.ResolveAnchorOutpoint(ctx, sourceID, account, message.AnchorIndex)
	if err != nil {
		return empty, err
	}
	anchorAddress, err := v.Source.AnchorAddress(ctx, anchorTxid, anchorVout)
	if err != nil {
		return empty, err
	}
	if anchorAddress == "" {
		return notFoundResponse(message), nil
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
	// OP_RETURN is free to write, but the anchor is not free to spend. Sources
	// that cannot report what input[0] spent leave the field empty and rely on
	// having searched by position instead.
	if batch.AnchorInputAddress != "" && batch.AnchorInputAddress != anchorAddress {
		return empty, fmt.Errorf(
			"batch %s spends %s as input[0], not anchor chain %d's address %s: %w",
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
	return buildResponse(message, status, received, batch)
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
