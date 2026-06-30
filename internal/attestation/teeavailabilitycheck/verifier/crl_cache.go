package verifier

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
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

// CRLCache fetches, caches, and returns CRLs keyed by CRL Distribution Point URL.
// Concurrent requests for the same URL are deduplicated via singleflight.
type CRLCache struct {
	mu      sync.RWMutex
	entries map[string]*crlEntry
	sfGroup singleflight.Group
	fetchFn func(ctx context.Context, url string, timeout time.Duration) ([]byte, error)
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

// getOrFetchCRL returns a cached CRL if fresh, otherwise fetches it.
// The issuer certificate is used to verify the CRL signature before caching.
func (c *CRLCache) getOrFetchCRL(ctx context.Context, url string, issuer *x509.Certificate) (*x509.RevocationList, error) {
	// Fast path: read lock
	c.mu.RLock()
	entry, ok := c.entries[url]
	c.mu.RUnlock()

	if ok && !isEntryStale(entry) {
		return entry.crl, nil
	}

	// Cache miss: deduplicate concurrent fetches for the same URL via singleflight.
	// The shared fetch runs under a background context (bounded by crlFetchTimeout
	// inside fetchFn), NOT the caller's context, so one caller's cancellation cannot
	// abort the in-flight fetch for the others. Each caller instead waits on its own
	// context via the DoChan result channel below.
	ch := c.sfGroup.DoChan(url, func() (any, error) {
		// Re-check cache — another goroutine may have populated it before singleflight acquired the key.
		c.mu.RLock()
		cachedEntry, exists := c.entries[url]
		c.mu.RUnlock()
		if exists && !isEntryStale(cachedEntry) {
			return cachedEntry.crl, nil
		}

		data, err := c.fetchFn(context.Background(), url, crlFetchTimeout)
		if err != nil {
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

		if err := crl.CheckSignatureFrom(issuer); err != nil {
			return nil, fmt.Errorf("CRL issuer verification failed: %w", err)
		}

		c.mu.Lock()
		if _, exists := c.entries[url]; !exists && len(c.entries) >= crlMaxEntries {
			c.evictStaleEntries()
			if len(c.entries) >= crlMaxEntries {
				c.evictOldestEntry()
			}
		}
		c.entries[url] = &crlEntry{
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
	for url, entry := range c.entries {
		if isEntryStale(entry) {
			delete(c.entries, url)
		}
	}
}

// evictOldestEntry removes the oldest cached entry. Must be called with mu held.
func (c *CRLCache) evictOldestEntry() {
	var (
		oldestURL  string
		oldestTime time.Time
		found      bool
	)
	for url, entry := range c.entries {
		if !found || entry.fetchedAt.Before(oldestTime) {
			oldestURL = url
			oldestTime = entry.fetchedAt
			found = true
		}
	}
	if found {
		delete(c.entries, oldestURL)
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
