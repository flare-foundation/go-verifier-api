// Package btcverifier verifies PMWPaymentStatus attestations for Bitcoin: it
// resolves a payment to its batch, locates the settling batch transaction in the
// verifier-utxo-indexer database, and matches the payment's output group against
// the on-chain instruction. It is the BTC source verifier for the shared
// PMWPaymentStatus attestation type (the XRP sibling lives in ../xrp).
package btcverifier

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/teepaymentsutxo"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
)

// onChainCallTimeout bounds a single eth_call, below the per-request verifier
// deadline, so a hung Flare RPC is abandoned quickly.
const onChainCallTimeout = 5 * time.Second

// Resolver reads the on-chain state the verifier needs to locate a batch: the
// batchPaymentId owning a payment (the BTC analog of the XRP getInitialNonce
// binding), and a chain's genesis anchor outpoint — whose address identifies the
// anchor chain in the indexer. An interface so tests can substitute a stub.
type Resolver interface {
	ResolveBatch(ctx context.Context, sourceID common.Hash, account string, paymentId uint64) (uint64, error)
	ResolveAnchorOutpoint(ctx context.Context, sourceID common.Hash, account string, anchorIndex uint32) (txid string, vout uint32, err error)
}

// utxoCaller is the subset of the generated TeePaymentsUtxo binding the resolver
// needs.
type utxoCaller interface {
	GetBatchPaymentId(opts *bind.CallOpts, account teepaymentsutxo.ITeePaymentsBasePMWMultisigAccount, paymentId uint64) (uint64, error)
	GetAnchor(opts *bind.CallOpts, account teepaymentsutxo.ITeePaymentsBasePMWMultisigAccount, anchorIndex *big.Int) (teepaymentsutxo.ITeePaymentsUtxoUtxoAnchorState, error)
}

// OnChainResolver resolves batch and anchor state via eth_calls to the source's
// TeePaymentsUtxo contract. These are per-payment lookups and are not cached.
type OnChainResolver struct {
	caller utxoCaller
	client *ethclient.Client // retained only for Close; nil when built from a caller (tests)
}

var (
	_ Resolver  = (*OnChainResolver)(nil)
	_ io.Closer = (*OnChainResolver)(nil)
)

// NewResolver wraps an already-built caller (used in tests with a stub).
func NewResolver(caller utxoCaller) *OnChainResolver {
	return &OnChainResolver{caller: caller}
}

// NewOnChainResolver dials the Flare RPC and binds against the source's
// TeePaymentsUtxo contract at contractAddr. Dial validates the URL but does not
// connect eagerly, so a node that is down fails per-request (fail closed), not
// at boot.
func NewOnChainResolver(rpcURL string, contractAddr common.Address) (*OnChainResolver, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Flare node at %s: %w", rpcURL, err)
	}
	caller, err := teepaymentsutxo.NewTeePaymentsUtxoCaller(contractAddr, client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("cannot create TeePaymentsUtxo caller at %s: %w", contractAddr.Hex(), err)
	}
	r := NewResolver(caller)
	r.client = client
	return r, nil
}

// Close releases the underlying RPC connection. Safe on a resolver built without
// one (tests) and safe to call more than once.
func (r *OnChainResolver) Close() error {
	if r.client != nil {
		r.client.Close()
	}
	return nil
}

// ResolveBatch returns the batchPaymentId owning paymentId. An unknown/never-
// issued paymentId reverts InvalidPaymentId() on-chain, surfaced as an error; an
// RPC failure also fails closed. Both wrap ErrDatabase.
func (r *OnChainResolver) ResolveBatch(ctx context.Context, sourceID common.Hash, account string, paymentId uint64) (uint64, error) {
	if paymentId == 0 {
		return 0, fmt.Errorf("paymentId must be >= 1: %w", paymentdb.ErrDatabase)
	}
	callCtx, cancel := context.WithTimeout(ctx, onChainCallTimeout)
	defer cancel()
	batchID, err := r.caller.GetBatchPaymentId(&bind.CallOpts{Context: callCtx}, teepaymentsutxo.ITeePaymentsBasePMWMultisigAccount{
		SourceId:       sourceID,
		AccountAddress: account,
	}, paymentId)
	if err != nil {
		return 0, fmt.Errorf("cannot resolve batchPaymentId for account %s paymentId %d: %v: %w", account, paymentId, err, paymentdb.ErrDatabase)
	}
	return batchID, nil
}

// ResolveAnchorOutpoint returns the genesis anchor outpoint (txid, vout) of the
// account's anchor chain anchorIndex. Its output address (resolved via the
// indexer) is the reused anchor address that identifies every batch on that
// chain. An RPC failure fails closed (wraps ErrDatabase).
func (r *OnChainResolver) ResolveAnchorOutpoint(ctx context.Context, sourceID common.Hash, account string, anchorIndex uint32) (string, uint32, error) {
	callCtx, cancel := context.WithTimeout(ctx, onChainCallTimeout)
	defer cancel()
	st, err := r.caller.GetAnchor(&bind.CallOpts{Context: callCtx}, teepaymentsutxo.ITeePaymentsBasePMWMultisigAccount{
		SourceId:       sourceID,
		AccountAddress: account,
	}, new(big.Int).SetUint64(uint64(anchorIndex)))
	if err != nil {
		return "", 0, fmt.Errorf("cannot resolve anchor %d for account %s: %v: %w", anchorIndex, account, err, paymentdb.ErrDatabase)
	}
	return txidFromBytes32(st.GenesisAnchorTxid), st.GenesisAnchorVout, nil
}

// txidFromBytes32 renders an on-chain 32-byte txid as the display-order hex string
// the indexer stores as transaction_id. Bitcoin displays txids in reverse byte
// order relative to their internal encoding, so the bytes are reversed here.
//
// NOTE: this assumes GenesisAnchorTxid is held in internal byte order on-chain;
// validate against a real registered anchor and drop the reversal if it is
// already stored in display order.
func txidFromBytes32(b [32]byte) string {
	// NO reversal. The genesis anchor outpoint reaches the chain through the
	// PMWMultisigUtxoConfigured request, whose verifier hex-encodes it straight
	// into `gettxout` — so what the registry stores is already DISPLAY order.
	// Reversing here asks the node about an outpoint that does not exist, and
	// the payment then attests as not-found with nothing to point at.
	return hex.EncodeToString(b[:])
}
