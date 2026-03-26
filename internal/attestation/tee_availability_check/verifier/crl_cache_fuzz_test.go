package verifier

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func FuzzGetOrFetchCRL(f *testing.F) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		f.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		f.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		f.Fatal(err)
	}

	wrongCAKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		f.Fatal(err)
	}
	wrongCADER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &wrongCAKey.PublicKey, wrongCAKey)
	if err != nil {
		f.Fatal(err)
	}
	wrongCACert, err := x509.ParseCertificate(wrongCADER)
	if err != nil {
		f.Fatal(err)
	}

	crlTemplate := &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: time.Now().Add(-time.Hour),
		NextUpdate: time.Now().Add(time.Hour),
	}
	validCRLDER, err := x509.CreateRevocationList(rand.Reader, crlTemplate, caCert, caKey)
	if err != nil {
		f.Fatal(err)
	}
	validCRLPEM := pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: validCRLDER})

	f.Add(validCRLDER)
	f.Add(validCRLPEM)
	f.Add([]byte("not a CRL"))
	f.Add([]byte{})
	f.Add([]byte{0x30, 0x82, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		const crlURL = "http://example.com/crl"

		cache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return data, nil
			},
		}

		crl, err := cache.getOrFetchCRL(context.Background(), crlURL, caCert)
		if err != nil {
			if _, ok := cache.entries[crlURL]; ok {
				t.Fatalf("failed CRL fetch cached entry for %q", crlURL)
			}
			return
		}

		if crl == nil {
			t.Fatal("successful CRL fetch returned nil CRL")
		}
		entry, ok := cache.entries[crlURL]
		if !ok || entry == nil || entry.crl == nil {
			t.Fatalf("successful CRL fetch did not populate cache for %q", crlURL)
		}
		if err := crl.CheckSignatureFrom(caCert); err != nil {
			t.Fatalf("successful CRL fetch returned CRL that failed issuer verification: %v", err)
		}

		wrongIssuerCache := &CRLCache{
			entries: make(map[string]*crlEntry),
			fetchFn: func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				return data, nil
			},
		}
		if _, err := wrongIssuerCache.getOrFetchCRL(context.Background(), crlURL, wrongCACert); err == nil {
			t.Fatal("accepted CRL for the wrong issuer")
		}
		if _, ok := wrongIssuerCache.entries[crlURL]; ok {
			t.Fatalf("wrong issuer CRL was cached for %q", crlURL)
		}
	})
}
