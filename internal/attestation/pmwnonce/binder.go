// Package pmwnonce binds a PMW payment's on-ledger XRP sequence to the value the
// TeePayments contract deterministically assigns it, closing the indexer-trust
// gap where the verifier would otherwise trust the sequence carried in an
// indexed instruction event.
package pmwnonce

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/teepayments"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
)

// rpcTimeout bounds a single getInitialNonce eth_call. The verifier runs under
// the inbound request context, but the HTTP server's write timeout does not bound
// verifier work, so a hung Flare RPC would otherwise pin request goroutines.
const rpcTimeout = 5 * time.Second

// maxInitialNonceCacheEntries bounds the initialNonce cache. initialNonce is
// immutable per account so entries never go stale (no TTL needed); the cap is
// defense-in-depth so a flood of requests for distinct (e.g. fabricated)
// accounts cannot grow the map without limit. On overflow the least-recently-used
// entry is evicted; a later request for an evicted account simply re-fetches it.
const maxInitialNonceCacheEntries = 100_000

// SequenceVerifier binds a payment's XRP sequence to its paymentId. Verifiers
// hold this interface so tests can substitute a stub for the on-chain Binder.
type SequenceVerifier interface {
	VerifySequence(ctx context.Context, sourceID common.Hash, account string, paymentId, gotSequence uint64) error
}

// initialNonceCaller is the subset of the generated TeePayments binding the
// binder needs: the immutable initial XRP sequence captured at account
// registration.
type initialNonceCaller interface {
	GetInitialNonce(opts *bind.CallOpts, account teepayments.ITeePaymentsBasePMWMultisigAccount) (uint64, error)
}

// Binder verifies that a payment's on-ledger XRP sequence equals the value the
// contract deterministically assigns it:
//
//	sequence = initialNonce + paymentId - 1
//
// initialNonce is TeePayments.getInitialNonce(account) — the account's XRP
// sequence captured at registration. It is immutable afterwards, so it is
// fetched once per account and cached in a bounded LRU (see
// maxInitialNonceCacheEntries).
type Binder struct {
	caller initialNonceCaller
	// client is the underlying RPC connection, retained only so Close can release
	// it. It is nil when the binder is built from a caller directly (tests).
	client *ethclient.Client
	// mu guards the LRU. A plain Mutex (not RWMutex) because a cache hit must move
	// its entry to the front, which mutates the list.
	mu    sync.Mutex
	ll    *list.List               // front = most recently used
	cache map[string]*list.Element // key -> element holding a *cacheEntry
}

// cacheEntry is the value stored in each LRU list element.
type cacheEntry struct {
	key   string
	nonce uint64
}

var (
	_ SequenceVerifier = (*Binder)(nil)
	_ io.Closer        = (*Binder)(nil)
)

// NewBinder wraps an already-built caller. Used in tests with a stub caller.
func NewBinder(caller initialNonceCaller) *Binder {
	return &Binder{
		caller: caller,
		ll:     list.New(),
		cache:  make(map[string]*list.Element),
	}
}

// Close releases the underlying RPC connection. Safe to call on a binder built
// without one (tests) and safe to call more than once.
func (b *Binder) Close() error {
	if b.client != nil {
		b.client.Close()
	}
	return nil
}

// NewOnChainBinder dials the Flare RPC and binds against the TeePayments facet at
// the FlareTeeManager diamond address. ethclient.Dial validates the URL but does
// not connect eagerly, so a node that is temporarily down fails per-request (and
// fails closed) rather than at boot.
func NewOnChainBinder(rpcURL string, contractAddr common.Address) (*Binder, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Flare node at %s: %w", rpcURL, err)
	}
	caller, err := teepayments.NewTeePaymentsCaller(contractAddr, client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("cannot create TeePayments caller at %s: %w", contractAddr.Hex(), err)
	}
	binder := NewBinder(caller)
	binder.client = client
	return binder, nil
}

// VerifySequence rejects the request unless gotSequence is exactly the sequence
// the contract assigns this paymentId (initialNonce + paymentId - 1). A mismatch
// means the indexed event's sequence disagrees with on-chain ground truth —
// corrupt or tampered indexer data — and fails closed. An RPC failure reading
// initialNonce also fails closed. Both wrap ErrDatabase.
func (b *Binder) VerifySequence(ctx context.Context, sourceID common.Hash, account string, paymentId, gotSequence uint64) error {
	if paymentId == 0 {
		return fmt.Errorf("paymentId must be >= 1: %w", paymentdb.ErrDatabase)
	}
	initial, err := b.initialNonce(ctx, sourceID, account)
	if err != nil {
		return err
	}
	// paymentId >= 1, so initialNonce + paymentId - 1 cannot underflow; guard the
	// addition against uint64 overflow before computing it.
	if initial > math.MaxUint64-(paymentId-1) {
		return fmt.Errorf("initialNonce %d + paymentId %d overflows uint64: %w", initial, paymentId, paymentdb.ErrDatabase)
	}
	want := initial + paymentId - 1
	if gotSequence != want {
		return fmt.Errorf("XRP sequence mismatch for paymentId %d: event Nonce %d != initialNonce %d + paymentId - 1 (= %d): %w", paymentId, gotSequence, initial, want, paymentdb.ErrDatabase)
	}
	return nil
}

func (b *Binder) initialNonce(ctx context.Context, sourceID common.Hash, account string) (uint64, error) {
	key := sourceID.Hex() + "|" + account
	if v, ok := b.cachedNonce(key); ok {
		return v, nil
	}
	// Fetch outside the lock: the RPC is slow and initialNonce is immutable, so a
	// rare concurrent double-fetch for the same account is harmless (idempotent).
	callCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	n, err := b.caller.GetInitialNonce(&bind.CallOpts{Context: callCtx}, teepayments.ITeePaymentsBasePMWMultisigAccount{
		SourceId:       sourceID,
		AccountAddress: account,
	})
	if err != nil {
		return 0, fmt.Errorf("cannot read initialNonce for account %s: %v: %w", account, err, paymentdb.ErrDatabase)
	}
	b.storeNonce(key, n)
	return n, nil
}

// cachedNonce returns the cached initialNonce for key, marking it most recently
// used on a hit.
func (b *Binder) cachedNonce(key string) (uint64, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	el, ok := b.cache[key]
	if !ok {
		return 0, false
	}
	b.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).nonce, true
}

// storeNonce inserts (or refreshes) key as most recently used, evicting the
// least-recently-used entry if the cache is over capacity.
func (b *Binder) storeNonce(key string, nonce uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if el, ok := b.cache[key]; ok {
		el.Value.(*cacheEntry).nonce = nonce
		b.ll.MoveToFront(el)
		return
	}
	b.cache[key] = b.ll.PushFront(&cacheEntry{key: key, nonce: nonce})
	if b.ll.Len() > maxInitialNonceCacheEntries {
		oldest := b.ll.Back()
		b.ll.Remove(oldest)
		delete(b.cache, oldest.Value.(*cacheEntry).key)
	}
}
