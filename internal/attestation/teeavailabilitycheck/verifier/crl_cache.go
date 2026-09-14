package verifier

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/fetcher"
	"golang.org/x/sync/singleflight"
)

const (
	crlFetchTimeout = 2 * time.Second
	crlMaxEntries   = 100
	crlMaxCacheTTL  = 4 * time.Hour
)

// crlEntry holds a cached CRL and the time it was fetched.
type crlEntry struct {
	crl       *x509.RevocationList
	fetchedAt time.Time
}

// CRLCache fetches, caches, and returns CRLs keyed by CRL Distribution Point URL
// AND issuer fingerprint (see crlCacheKey). Concurrent requests for the same key
// are deduplicated via singleflight.
type CRLCache struct {
	mu      sync.RWMutex
	entries map[string]*crlEntry
	sfGroup singleflight.Group
	fetchFn func(ctx context.Context, url string, timeout time.Duration) ([]byte, error)
}

// crlCacheKey scopes a cache/singleflight entry to the distribution-point URL
// AND the exact issuer certificate the CRL must verify against — the URL alone
// would hand a CRL cached under one issuer to a different chain sharing the
// URL. The fingerprint hashes the whole certificate, not just its key:
// CheckSignatureFrom verifies the KEY and never compares issuer names, so a
// key-only fingerprint would collapse distinct issuers that share a key.
func crlCacheKey(url string, issuer *x509.Certificate) string {
	sum := sha256.Sum256(issuer.Raw)
	return fmt.Sprintf("%s|%x", url, sum)
}

// verifyCRLIssuer binds a CRL to the exact issuer certificate: the CRL's
// issuer NAME must be the certificate's subject — CheckSignatureFrom verifies
// only the key, so two issuers sharing one would otherwise vouch for each
// other's CRLs — and the signature must verify.
func verifyCRLIssuer(crl *x509.RevocationList, issuer *x509.Certificate) error {
	if !bytes.Equal(crl.RawIssuer, issuer.RawSubject) {
		return fmt.Errorf("CRL issuer %q is not the certificate subject %q", crl.Issuer, issuer.Subject)
	}
	return crl.CheckSignatureFrom(issuer)
}

// NewCRLCache creates a CRLCache that fetches CRLs through SSRF-safe pinned URL resolution.
func NewCRLCache() *CRLCache {
	return &CRLCache{
		entries: make(map[string]*crlEntry),
		fetchFn: fetchCRLBytes,
	}
}

// fetchCRLBytes resolves the URL via ResolveExternalURL with allowPrivateNetworks=false
// (CRL distribution points must never resolve to private/local addresses, independent of
// ALLOW_PRIVATE_NETWORKS which is scoped to the TEE proxy), then fetches the body via a
// connection pinned to the resolved IP.
func fetchCRLBytes(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	resolved, err := ResolveExternalURL(ctx, url, false)
	if err != nil {
		return nil, fmt.Errorf("resolving CRL URL %s: %w", url, err)
	}
	dialAddr, hostHeader, serverName := BuildPinnedAddr(resolved)
	return fetcher.FetchBytesPinned(ctx, url, timeout, dialAddr, hostHeader, serverName)
}

// FetchCRLsForToken parses the attestation token without verification to extract
// x5c certificates, then fetches/caches CRLs for the leaf and intermediate certificates.
// The expectedRoot is the trusted root certificate — the token's root must match and the
// full chain (intermediate signed by root, leaf signed by intermediate, validity windows
// current) must be valid before any CRL distribution point URL is dereferenced.
func (c *CRLCache) FetchCRLsForToken(ctx context.Context, attestationToken string, expectedRoot *x509.Certificate) (leafCRL, intermediateCRL *x509.RevocationList, err error) {
	token, _, err := googlecloud.ParsePKITokenUnverified(attestationToken)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing unverified token: %w", err)
	}

	x5cs, ok := token.Header["x5c"]
	if !ok {
		return nil, nil, errors.New("x5c header missing from token")
	}
	x5cHeaders, ok := x5cs.([]any)
	if !ok {
		return nil, nil, errors.New("x5c header is not a slice")
	}

	certs, err := googlecloud.ExtractCertificatesFromX5CHeader(x5cHeaders)
	if err != nil {
		return nil, nil, fmt.Errorf("extracting certificates: %w", err)
	}

	if expectedRoot != nil && !expectedRoot.Equal(certs.Root) {
		return nil, nil, errors.New("token root certificate does not match trusted root")
	}

	// Pre-validate the x5c chain (signature + validity windows) before any
	// CRL URL is dereferenced. Signature-only — CRLs are not yet fetched,
	// so revocation is checked downstream in ParseAndValidatePKIToken.
	// Rejecting bad chains here prevents attacker-supplied certs (with
	// arbitrary CRL distribution point URLs) from triggering outbound
	// requests.
	if err := validateX5CChain(certs.Root, certs.Intermediate, certs.Leaf); err != nil {
		return nil, nil, fmt.Errorf("x5c chain validation failed: %w", err)
	}

	type crlResult struct {
		crl *x509.RevocationList
		err error
	}

	leafCh := make(chan crlResult, 1)
	intermediateCh := make(chan crlResult, 1)

	go func() {
		crl, err := c.fetchFirstCRL(ctx, "leaf", certs.Leaf.CRLDistributionPoints, certs.Intermediate)
		leafCh <- crlResult{crl, err}
	}()

	go func() {
		crl, err := c.fetchFirstCRL(ctx, "intermediate", certs.Intermediate.CRLDistributionPoints, certs.Root)
		intermediateCh <- crlResult{crl, err}
	}()

	leafRes := <-leafCh
	intermediateRes := <-intermediateCh

	if leafRes.err != nil {
		return nil, nil, leafRes.err
	}
	if intermediateRes.err != nil {
		return nil, nil, intermediateRes.err
	}

	return leafRes.crl, intermediateRes.crl, nil
}

// fetchFirstCRL iterates over CRL distribution points and returns the first
// successfully fetched and issuer-verified CRL. Returns (nil, nil) if the list is empty.
func (c *CRLCache) fetchFirstCRL(ctx context.Context, certName string, distributionPoints []string, issuer *x509.Certificate) (*x509.RevocationList, error) {
	if len(distributionPoints) == 0 {
		return nil, nil
	}

	var errs []error
	for _, url := range distributionPoints {
		crl, err := c.getOrFetchCRL(ctx, url, issuer)
		if err != nil {
			logger.Warnf("Failed to fetch %s CRL from %s: %v", certName, url, err)
			errs = append(errs, fmt.Errorf("%s: %w", url, err))
			continue
		}
		return crl, nil
	}

	return nil, fmt.Errorf("fetching %s CRL failed for all distribution points: %w", certName, errors.Join(errs...))
}

// isTransientFetchError reports whether a CRL fetch failure is transient (→
// retryable). Only transport outages, fetch timeouts, and 5xx responses qualify;
// deterministic failures (invalid/unresolvable URL, refused redirect, 404, other
// 4xx, oversized body) do not, so they are rejected rather than retried forever.
func isTransientFetchError(err error) bool {
	// These surface through the HTTP client — some even wrap ErrHTTPFetch — but are
	// deterministic, so they must be excluded before the ErrHTTPFetch transport case.
	if errors.Is(err, fetcher.ErrRedirect) ||
		errors.Is(err, fetcher.ErrNotFound) ||
		errors.Is(err, fetcher.ErrResponseTooLarge) {
		return false
	}
	// A non-2xx status: 5xx is a server-side hiccup that may recover, and the
	// retryable 4xx (408 Request Timeout, 429 Too Many Requests) likewise; other
	// 4xx are deterministic. HTTPStatusError.Unwrap is ErrHTTPFetch, so this must be
	// checked before the transport case below.
	var httpErr *fetcher.HTTPStatusError
	if errors.As(err, &httpErr) {
		return (httpErr.Code >= 500 && httpErr.Code < 600) ||
			httpErr.Code == http.StatusRequestTimeout ||
			httpErr.Code == http.StatusTooManyRequests
	}
	// A temporary or timed-out DNS resolution (surfaced wrapped in ErrURLValidation)
	// is transient; a deterministic resolution failure (NXDOMAIN, SSRF-blocked,
	// invalid URL) is not.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsTemporary || dnsErr.IsTimeout
	}
	// Genuine transport failure (connection/TLS/read, incl. a dropped body read now
	// tagged ErrHTTPFetch) or a fetch timeout/cancellation.
	return errors.Is(err, fetcher.ErrHTTPFetch) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

// cachedFresh returns a fresh cache hit for key, or nil on a miss. A hit is
// re-verified against the caller's issuer before it is handed out (defense in
// depth on top of the issuer-scoped key): a cached CRL that does not verify
// against THIS issuer is an error, never an answer.
func (c *CRLCache) cachedFresh(key string, issuer *x509.Certificate) (*x509.RevocationList, error) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || isEntryStale(entry) {
		return nil, nil
	}
	if err := verifyCRLIssuer(entry.crl, issuer); err != nil {
		return nil, fmt.Errorf("cached CRL does not verify against the supplied issuer: %w", err)
	}
	return entry.crl, nil
}

// getOrFetchCRL returns a cached CRL if fresh, otherwise fetches it. Entries are
// scoped to (URL, issuer) — see crlCacheKey — and the issuer certificate is used
// to verify the CRL signature before caching and on every hit.
func (c *CRLCache) getOrFetchCRL(ctx context.Context, url string, issuer *x509.Certificate) (*x509.RevocationList, error) {
	// No issuer means nothing could ever verify the CRL — fail closed before
	// keying the cache or dereferencing the network.
	if issuer == nil {
		return nil, errors.New("an issuer certificate is required to fetch a CRL")
	}
	key := crlCacheKey(url, issuer)

	// Fast path: read lock
	if crl, err := c.cachedFresh(key, issuer); err != nil {
		return nil, err
	} else if crl != nil {
		return crl, nil
	}

	// Cache miss: deduplicate concurrent fetches for the same key via singleflight.
	// The shared fetch runs under a background context (bounded by crlFetchTimeout
	// inside fetchFn), NOT the caller's context, so one caller's cancellation cannot
	// abort the in-flight fetch for the others. Each caller instead waits on its own
	// context via the DoChan result channel below.
	ch := c.sfGroup.DoChan(key, func() (any, error) {
		// Re-check cache — another goroutine may have populated it before singleflight acquired the key.
		if crl, err := c.cachedFresh(key, issuer); err != nil {
			return nil, err
		} else if crl != nil {
			return crl, nil
		}

		data, err := c.fetchFn(context.Background(), url, crlFetchTimeout)
		if err != nil {
			// Only a genuinely transient fetch failure (transport outage, timeout,
			// 5xx) is retryable. A deterministic failure — invalid/unresolvable URL,
			// refused redirect, 404, other 4xx, oversized body — is a bad or
			// misconfigured distribution point and must be rejected, not retried
			// forever. (Parse, issuer verification and NextUpdate below, plus every
			// attestation-level check in FetchCRLsForToken, are likewise deterministic
			// and stay untagged.)
			if isTransientFetchError(err) {
				return nil, fmt.Errorf("fetching CRL: %w: %w", ErrTEERevocationUnavailable, err)
			}
			return nil, fmt.Errorf("fetching CRL: %w", err)
		}

		// Try PEM decode first (Google Cloud CRLs are PEM-encoded), fall back to raw DER.
		if block, _ := pem.Decode(data); block != nil {
			data = block.Bytes
		}

		crl, err := x509.ParseRevocationList(data)
		if err != nil {
			return nil, fmt.Errorf("parsing CRL: %w", err)
		}

		if err := verifyCRLIssuer(crl, issuer); err != nil {
			return nil, fmt.Errorf("CRL issuer verification failed: %w", err)
		}

		// A CRL without a NextUpdate has no defined validity horizon. Refuse to
		// trust one: the cache would refetch it on every call (isEntryStale
		// treats a zero NextUpdate as stale) and downstream validation would
		// otherwise accept it indefinitely. Fail closed to a CRL-fetch error.
		if crl.NextUpdate.IsZero() {
			return nil, fmt.Errorf("CRL from %s has no NextUpdate; refusing a CRL without a validity horizon", url)
		}

		c.mu.Lock()
		if _, exists := c.entries[key]; !exists && len(c.entries) >= crlMaxEntries {
			c.evictStaleEntries()
			if len(c.entries) >= crlMaxEntries {
				c.evictOldestEntry()
			}
		}
		c.entries[key] = &crlEntry{
			crl:       crl,
			fetchedAt: time.Now(),
		}
		c.mu.Unlock()

		logger.Infof("Fetched and cached CRL from %s (NextUpdate: %s)", url, crl.NextUpdate.Format(time.RFC3339))
		return crl, nil
	})

	select {
	case <-ctx.Done():
		// This caller gave up, but the shared fetch keeps running and still populates
		// the cache for the other waiters (and this caller's next attempt).
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		crl, ok := res.Val.(*x509.RevocationList)
		if !ok {
			return nil, fmt.Errorf("unexpected singleflight result type: %T", res.Val)
		}
		// Same defense in depth as a cache hit: the key ties every waiter to one
		// issuer certificate, but a shared result is still never handed out
		// unverified against THIS caller's issuer.
		if err := verifyCRLIssuer(crl, issuer); err != nil {
			return nil, fmt.Errorf("shared CRL result does not verify against the supplied issuer: %w", err)
		}
		return crl, nil
	}
}

// isEntryStale returns true if the entry has exceeded crlMaxCacheTTL or the CRL's NextUpdate has passed.
func isEntryStale(entry *crlEntry) bool {
	now := time.Now()
	if now.After(entry.fetchedAt.Add(crlMaxCacheTTL)) {
		return true
	}
	if entry.crl.NextUpdate.IsZero() {
		return true
	}
	return now.After(entry.crl.NextUpdate)
}

// evictStaleEntries removes stale entries from the cache. Must be called with mu held.
func (c *CRLCache) evictStaleEntries() {
	for key, entry := range c.entries {
		if isEntryStale(entry) {
			delete(c.entries, key)
		}
	}
}

// evictOldestEntry removes the oldest cached entry. Must be called with mu held.
func (c *CRLCache) evictOldestEntry() {
	var (
		oldestKey  string
		oldestTime time.Time
		found      bool
	)
	for key, entry := range c.entries {
		if !found || entry.fetchedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.fetchedAt
			found = true
		}
	}
	if found {
		delete(c.entries, oldestKey)
	}
}

// Close clears the CRL cache.
func (c *CRLCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*crlEntry)
	return nil
}

// validateX5CChain checks intermediate is signed by root, leaf is signed by
// intermediate, and all three are within their validity windows. Signature-only
// — revocation is not checked here (CRLs are fetched downstream).
func validateX5CChain(root, intermediate, leaf *x509.Certificate) error {
	now := time.Now()
	if err := checkCertValidity(root, "root", now); err != nil {
		return err
	}
	if err := checkCertValidity(intermediate, "intermediate", now); err != nil {
		return err
	}
	if err := checkCertValidity(leaf, "leaf", now); err != nil {
		return err
	}
	if err := intermediate.CheckSignatureFrom(root); err != nil {
		return fmt.Errorf("intermediate not signed by root: %w", err)
	}
	if err := leaf.CheckSignatureFrom(intermediate); err != nil {
		return fmt.Errorf("leaf not signed by intermediate: %w", err)
	}
	return nil
}

func checkCertValidity(cert *x509.Certificate, name string, now time.Time) error {
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("%s certificate not yet valid (NotBefore=%s)", name, cert.NotBefore.Format(time.RFC3339))
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("%s certificate expired (NotAfter=%s)", name, cert.NotAfter.Format(time.RFC3339))
	}
	return nil
}
