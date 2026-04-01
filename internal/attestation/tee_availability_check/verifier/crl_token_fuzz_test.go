package verifier

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func FuzzFetchCRLsForToken(f *testing.F) {
	const (
		leafCRLURL         = "http://example.com/leaf.crl"
		intermediateCRLURL = "http://example.com/intermediate.crl"
	)

	rootCert, rootKey, err := generateFuzzCert(true, nil, nil, nil)
	if err != nil {
		f.Fatal(err)
	}
	intermediateCert, intermediateKey, err := generateFuzzCert(true, rootCert, rootKey, []string{intermediateCRLURL})
	if err != nil {
		f.Fatal(err)
	}
	leafCert, leafKey, err := generateFuzzCert(false, intermediateCert, intermediateKey, []string{leafCRLURL})
	if err != nil {
		f.Fatal(err)
	}

	x5c := []string{
		base64.StdEncoding.EncodeToString(leafCert.Raw),
		base64.StdEncoding.EncodeToString(intermediateCert.Raw),
		base64.StdEncoding.EncodeToString(rootCert.Raw),
	}
	validToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{})
	validToken.Header["x5c"] = x5c
	validTokenSigned, err := validToken.SignedString(leafKey)
	if err != nil {
		f.Fatal(err)
	}

	noX5CToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{})
	noX5CTokenSigned, err := noX5CToken.SignedString(leafKey)
	if err != nil {
		f.Fatal(err)
	}

	leafCRLBytes, err := createFuzzCRL(intermediateCert, intermediateKey, time.Now().Add(time.Hour))
	if err != nil {
		f.Fatal(err)
	}
	intermediateCRLBytes, err := createFuzzCRL(rootCert, rootKey, time.Now().Add(time.Hour))
	if err != nil {
		f.Fatal(err)
	}

	wrongRoot, _, err := generateFuzzCert(true, nil, nil, nil)
	if err != nil {
		f.Fatal(err)
	}

	f.Add(validTokenSigned, uint8(0))
	f.Add(validTokenSigned, uint8(1))
	f.Add(validTokenSigned, uint8(2))
	f.Add(noX5CTokenSigned, uint8(0))
	f.Add("not-a-jwt", uint8(0))
	f.Add("eyJhbGciOiJub25lIn0.e30.", uint8(0))
	f.Add("", uint8(0))

	f.Fuzz(func(t *testing.T, token string, rootMode uint8) {
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

		var expectedRoot *x509.Certificate
		switch rootMode % 3 {
		case 1:
			expectedRoot = rootCert
		case 2:
			expectedRoot = wrongRoot
		}

		leafCRL, intermediateCRL, err := cache.FetchCRLsForToken(context.Background(), token, expectedRoot)
		if err != nil {
			if leafCRL != nil || intermediateCRL != nil {
				t.Fatal("failed token parsing returned non-nil CRLs")
			}
			return
		}

		if expectedRoot == wrongRoot {
			t.Fatal("token unexpectedly succeeded with the wrong expected root")
		}
		if leafCRL == nil || intermediateCRL == nil {
			t.Fatal("successful CRL extraction returned nil CRLs")
		}
	})
}

func generateFuzzCert(isCA bool, parent *x509.Certificate, parentKey *rsa.PrivateKey, crlDistPoints []string) (*x509.Certificate, *rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

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
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}
	return cert, priv, nil
}

func createFuzzCRL(issuer *x509.Certificate, issuerKey *rsa.PrivateKey, nextUpdate time.Time) ([]byte, error) {
	template := &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: time.Now().Add(-time.Hour),
		NextUpdate: nextUpdate,
	}
	return x509.CreateRevocationList(rand.Reader, template, issuer, issuerKey)
}
