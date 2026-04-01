package verifier

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// createTestCRL creates a signed CRL issued by the given CA with the specified nextUpdate.
//
//nolint:unparam // test helper designed to accept revoked serials
func createTestCRL(t *testing.T, issuer *x509.Certificate, issuerKey *rsa.PrivateKey, nextUpdate time.Time, revokedSerials ...*big.Int) []byte {
	t.Helper()
	revoked := make([]x509.RevocationListEntry, 0, len(revokedSerials))
	for _, serial := range revokedSerials {
		revoked = append(revoked, x509.RevocationListEntry{
			SerialNumber:   serial,
			RevocationTime: time.Now().Add(-time.Hour),
		})
	}
	template := &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now().Add(-time.Hour),
		NextUpdate:                nextUpdate,
		RevokedCertificateEntries: revoked,
	}
	crlBytes, err := x509.CreateRevocationList(rand.Reader, template, issuer, issuerKey)
	require.NoError(t, err)
	return crlBytes
}

// generateTestCert creates a self-signed or CA-signed certificate with optional CRL distribution points.
func generateTestCert(t *testing.T, isCA bool, parent *x509.Certificate, parentKey *rsa.PrivateKey, crlDistPoints []string) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		SignatureAlgorithm:    x509.SHA256WithRSA,
		IsCA:                  isCA,
		BasicConstraintsValid: true,
		CRLDistributionPoints: crlDistPoints,
	}
	if isCA {
		template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	} else {
		template.KeyUsage = x509.KeyUsageDigitalSignature
		template.Subject = pkix.Name{CommonName: "leaf"}
	}

	signer := parentKey
	signerCert := parent
	if parent == nil {
		signer = priv
		signerCert = template
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, signerCert, &priv.PublicKey, signer)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return cert, priv
}

// buildTestTokenWithCRLDists creates a signed JWT with x5c header containing certs that have the given CRL distribution points.
func buildTestTokenWithCRLDists(t *testing.T, leafCRLDists, intermediateCRLDists []string) (string, *x509.Certificate, *rsa.PrivateKey, *x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	rootCert, rootKey := generateTestCert(t, true, nil, nil, nil)
	intermediateCert, intermediateKey := generateTestCert(t, true, rootCert, rootKey, intermediateCRLDists)
	leafCert, leafKey := generateTestCert(t, false, intermediateCert, intermediateKey, leafCRLDists)

	x5c := []string{
		base64.StdEncoding.EncodeToString(leafCert.Raw),
		base64.StdEncoding.EncodeToString(intermediateCert.Raw),
		base64.StdEncoding.EncodeToString(rootCert.Raw),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{})
	token.Header["x5c"] = x5c
	signedToken, err := token.SignedString(leafKey)
	require.NoError(t, err)

	return signedToken, intermediateCert, intermediateKey, rootCert, rootKey
}

func TestIsEntryStale(t *testing.T) {
	t.Run("fresh entry", func(t *testing.T) {
		entry := &crlEntry{
			crl:       &x509.RevocationList{NextUpdate: time.Now().Add(time.Hour)},
			fetchedAt: time.Now(),
		}
		require.False(t, isEntryStale(entry))
	})
	t.Run("NextUpdate passed", func(t *testing.T) {
		entry := &crlEntry{
			crl:       &x509.RevocationList{NextUpdate: time.Now().Add(-time.Hour)},
			fetchedAt: time.Now(),
		}
		require.True(t, isEntryStale(entry))
	})
	t.Run("zero NextUpdate", func(t *testing.T) {
		entry := &crlEntry{
			crl:       &x509.RevocationList{},
			fetchedAt: time.Now(),
		}
		require.True(t, isEntryStale(entry))
	})
	t.Run("max TTL exceeded even if NextUpdate is far", func(t *testing.T) {
		entry := &crlEntry{
			crl:       &x509.RevocationList{NextUpdate: time.Now().Add(7 * 24 * time.Hour)},
			fetchedAt: time.Now().Add(-crlMaxCacheTTL - time.Minute),
		}
		require.True(t, isEntryStale(entry))
	})
	t.Run("within max TTL with far NextUpdate", func(t *testing.T) {
		entry := &crlEntry{
			crl:       &x509.RevocationList{NextUpdate: time.Now().Add(7 * 24 * time.Hour)},
			fetchedAt: time.Now().Add(-crlMaxCacheTTL + time.Hour),
		}
		require.False(t, isEntryStale(entry))
	})
}

func TestGetOrFetchCRL(t *testing.T) {
	t.Run("cache miss then hit", func(t *testing.T) {
		// Create a CA to sign the CRL
		caCert, caKey := generateTestCert(t, true, nil, nil, nil)
		crlBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

		fetchCount := 0
		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				fetchCount++
				return crlBytes, nil
			},
		}

		// First call: cache miss, should fetch
		crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", caCert)
		require.NoError(t, err)
		require.NotNil(t, crl)
		require.Equal(t, 1, fetchCount)

		// Second call: cache hit, should NOT fetch
		crl2, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", caCert)
		require.NoError(t, err)
		require.NotNil(t, crl2)
		require.Equal(t, 1, fetchCount)
	})

	t.Run("stale entry triggers refetch", func(t *testing.T) {
		caCert, caKey := generateTestCert(t, true, nil, nil, nil)
		freshCRLBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

		fetchCount := 0
		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				fetchCount++
				return freshCRLBytes, nil
			},
		}

		// Seed a stale entry
		staleCRL := &x509.RevocationList{NextUpdate: time.Now().Add(-time.Hour)}
		cache.entries["http://example.com/crl"] = &crlEntry{
			crl:       staleCRL,
			fetchedAt: time.Now().Add(-2 * time.Hour),
		}

		// Should refetch because stale
		crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", caCert)
		require.NoError(t, err)
		require.NotNil(t, crl)
		require.Equal(t, 1, fetchCount)
	})

	t.Run("fetch error", func(t *testing.T) {
		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return nil, errors.New("network error")
			},
		}

		crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", nil)
		require.ErrorContains(t, err, "fetching CRL")
		require.Nil(t, crl)
	})

	t.Run("parse error", func(t *testing.T) {
		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return []byte("not a valid CRL"), nil
			},
		}

		crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", nil)
		require.ErrorContains(t, err, "parsing CRL")
		require.Nil(t, crl)
	})

	t.Run("PEM-encoded CRL", func(t *testing.T) {
		caCert, caKey := generateTestCert(t, true, nil, nil, nil)
		derBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: derBytes})

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return pemBytes, nil
			},
		}

		crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", caCert)
		require.NoError(t, err)
		require.NotNil(t, crl)
	})

	t.Run("CRL signed by wrong issuer", func(t *testing.T) {
		caCert, caKey := generateTestCert(t, true, nil, nil, nil)
		wrongCACert, _ := generateTestCert(t, true, nil, nil, nil)
		crlBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return crlBytes, nil
			},
		}

		crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", wrongCACert)
		require.ErrorContains(t, err, "CRL issuer verification failed")
		require.Nil(t, crl)
		// Should not be cached
		require.Empty(t, cache.entries)
	})

	t.Run("concurrent calls deduplicated via singleflight", func(t *testing.T) {
		caCert, caKey := generateTestCert(t, true, nil, nil, nil)
		crlBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))

		var fetchCount int64
		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				atomic.AddInt64(&fetchCount, 1)
				time.Sleep(50 * time.Millisecond) // simulate network latency
				return crlBytes, nil
			},
		}

		const goroutines = 10
		var wg sync.WaitGroup
		wg.Add(goroutines)
		for range goroutines {
			go func() {
				defer wg.Done()
				crl, err := cache.getOrFetchCRL(context.Background(), "http://example.com/crl", caCert)
				require.NoError(t, err)
				require.NotNil(t, crl)
			}()
		}
		wg.Wait()

		require.Equal(t, int64(1), atomic.LoadInt64(&fetchCount), "singleflight should deduplicate concurrent fetches to a single call")
	})

	t.Run("evicts oldest entry when cache is full and all entries are fresh", func(t *testing.T) {
		caCert, caKey := generateTestCert(t, true, nil, nil, nil)
		crlBytes := createTestCRL(t, caCert, caKey, time.Now().Add(time.Hour))
		crl, err := x509.ParseRevocationList(crlBytes)
		require.NoError(t, err)

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return crlBytes, nil
			},
		}

		oldestURL := "http://example.com/oldest.crl"
		now := time.Now()
		for i := range crlMaxEntries - 1 {
			url := "http://example.com/crl/" + strconv.Itoa(i)
			fetchedAt := now.Add(time.Duration(i) * time.Minute)
			cache.entries[url] = &crlEntry{
				crl:       crl,
				fetchedAt: fetchedAt,
			}
		}
		cache.entries[oldestURL] = &crlEntry{
			crl:       crl,
			fetchedAt: now.Add(-time.Hour),
		}

		_, err = cache.getOrFetchCRL(context.Background(), "http://example.com/new.crl", caCert)
		require.NoError(t, err)
		require.Len(t, cache.entries, crlMaxEntries)
		require.NotContains(t, cache.entries, oldestURL)
		require.Contains(t, cache.entries, "http://example.com/new.crl")
	})
}

func TestGetCRLsForToken(t *testing.T) {
	t.Run("full flow with CRL distribution points", func(t *testing.T) {
		leafCRLURL := "http://example.com/leaf.crl"
		intermediateCRLURL := "http://example.com/intermediate.crl"

		signedToken, intermediateCert, intermediateKey, rootCert, rootKey := buildTestTokenWithCRLDists(t,
			[]string{leafCRLURL},
			[]string{intermediateCRLURL},
		)

		// Create CRLs signed by appropriate issuers
		leafCRLBytes := createTestCRL(t, intermediateCert, intermediateKey, time.Now().Add(time.Hour))
		intermediateCRLBytes := createTestCRL(t, rootCert, rootKey, time.Now().Add(time.Hour))

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				switch url {
				case leafCRLURL:
					return leafCRLBytes, nil
				case intermediateCRLURL:
					return intermediateCRLBytes, nil
				default:
					return nil, errors.New("unexpected URL: " + url)
				}
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.NoError(t, err)
		require.NotNil(t, leafCRL)
		require.NotNil(t, intermediateCRL)
	})

	t.Run("no CRL distribution points", func(t *testing.T) {
		signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t, nil, nil)

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				t.Fatal("fetchFn should not be called when no CRL distribution points")
				return nil, nil
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.NoError(t, err)
		require.Nil(t, leafCRL)
		require.Nil(t, intermediateCRL)
	})

	t.Run("leaf CRL fetch failure", func(t *testing.T) {
		signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t,
			[]string{"http://example.com/leaf.crl"},
			nil,
		)

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return nil, errors.New("connection refused")
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.ErrorContains(t, err, "fetching leaf CRL failed for all distribution points")
		require.Nil(t, leafCRL)
		require.Nil(t, intermediateCRL)
	})

	t.Run("intermediate CRL fetch failure", func(t *testing.T) {
		signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t,
			nil,
			[]string{"http://example.com/intermediate.crl"},
		)

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return nil, errors.New("connection refused")
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.ErrorContains(t, err, "fetching intermediate CRL failed for all distribution points")
		require.Nil(t, leafCRL)
		require.Nil(t, intermediateCRL)
	})

	t.Run("both CRL fetches fail", func(t *testing.T) {
		signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t,
			[]string{"http://example.com/leaf.crl"},
			[]string{"http://example.com/intermediate.crl"},
		)

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return nil, errors.New("connection refused")
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.Error(t, err)
		// Either leaf or intermediate error is returned (leaf checked first)
		errMsg := err.Error()
		require.True(t,
			strings.Contains(errMsg, "fetching leaf CRL") || strings.Contains(errMsg, "fetching intermediate CRL"),
			"expected CRL fetch error, got: %v", err,
		)
		require.Nil(t, leafCRL)
		require.Nil(t, intermediateCRL)
	})

	t.Run("multiple distribution points - first fails, second succeeds", func(t *testing.T) {
		failURL := "http://example.com/fail.crl"
		validLeafURL := "http://example.com/leaf.crl"
		validIntermediateURL := "http://example.com/intermediate.crl"

		signedToken, intermediateCert, intermediateKey, rootCert, rootKey := buildTestTokenWithCRLDists(t,
			[]string{failURL, validLeafURL},
			[]string{failURL, validIntermediateURL},
		)

		leafCRLBytes := createTestCRL(t, intermediateCert, intermediateKey, time.Now().Add(time.Hour))
		intermediateCRLBytes := createTestCRL(t, rootCert, rootKey, time.Now().Add(time.Hour))

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				switch url {
				case failURL:
					return nil, errors.New("connection refused")
				case validLeafURL:
					return leafCRLBytes, nil
				case validIntermediateURL:
					return intermediateCRLBytes, nil
				default:
					return nil, errors.New("unexpected URL: " + url)
				}
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.NoError(t, err)
		require.NotNil(t, leafCRL)
		require.NotNil(t, intermediateCRL)
	})

	t.Run("multiple distribution points - invalid CRL then valid CRL", func(t *testing.T) {
		badURL := "http://example.com/bad.crl"
		validURL := "http://example.com/leaf.crl"

		signedToken, intermediateCert, intermediateKey, _, _ := buildTestTokenWithCRLDists(t,
			[]string{badURL, validURL},
			nil,
		)

		leafCRLBytes := createTestCRL(t, intermediateCert, intermediateKey, time.Now().Add(time.Hour))

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				switch url {
				case badURL:
					return []byte("not a valid CRL"), nil
				case validURL:
					return leafCRLBytes, nil
				default:
					return nil, errors.New("unexpected URL: " + url)
				}
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.NoError(t, err)
		require.NotNil(t, leafCRL)
		require.Nil(t, intermediateCRL)
	})

	t.Run("multiple distribution points - all fail", func(t *testing.T) {
		signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t,
			[]string{"http://example.com/a.crl", "http://example.com/b.crl", "http://example.com/c.crl"},
			nil,
		)

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return nil, errors.New("connection refused")
			},
		}

		leafCRL, intermediateCRL, err := cache.GetCRLsForToken(context.Background(), signedToken, nil)
		require.ErrorContains(t, err, "fetching leaf CRL failed for all distribution points")
		require.ErrorContains(t, err, "http://example.com/a.crl")
		require.ErrorContains(t, err, "http://example.com/c.crl")
		require.Nil(t, leafCRL)
		require.Nil(t, intermediateCRL)
	})

	t.Run("root certificate mismatch", func(t *testing.T) {
		signedToken, _, _, _, _ := buildTestTokenWithCRLDists(t, nil, nil)
		wrongRoot, _ := generateTestCert(t, true, nil, nil, nil)

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				t.Fatal("fetchFn should not be called when root does not match")
				return nil, nil
			},
		}

		_, _, err := cache.GetCRLsForToken(context.Background(), signedToken, wrongRoot)
		require.ErrorContains(t, err, "token root certificate does not match trusted root")
	})

	t.Run("invalid token", func(t *testing.T) {
		cache := NewCRLCache()
		_, _, err := cache.GetCRLsForToken(context.Background(), "not-a-jwt", nil)
		require.ErrorContains(t, err, "parsing unverified token")
	})
}

func TestEvictStaleEntries(t *testing.T) {
	cache := &CRLCache{
		entries: make(map[string]*crlEntry),
	}

	// Add a fresh entry
	cache.entries["fresh"] = &crlEntry{
		crl:       &x509.RevocationList{NextUpdate: time.Now().Add(time.Hour)},
		fetchedAt: time.Now(),
	}
	// Add a stale entry
	cache.entries["stale"] = &crlEntry{
		crl:       &x509.RevocationList{NextUpdate: time.Now().Add(-time.Hour)},
		fetchedAt: time.Now().Add(-2 * time.Hour),
	}

	cache.evictStaleEntries()

	require.Len(t, cache.entries, 1)
	require.Contains(t, cache.entries, "fresh")
	require.NotContains(t, cache.entries, "stale")
}

func TestCRLCacheClose(t *testing.T) {
	cache := NewCRLCache()
	cache.entries["test"] = &crlEntry{
		crl:       &x509.RevocationList{NextUpdate: time.Now().Add(time.Hour)},
		fetchedAt: time.Now(),
	}
	require.Len(t, cache.entries, 1)

	err := cache.Close()
	require.NoError(t, err)
	require.Empty(t, cache.entries)
}
