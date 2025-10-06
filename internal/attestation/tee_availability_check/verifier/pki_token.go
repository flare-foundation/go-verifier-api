package verifier

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"

	teenodetype "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/golang-jwt/jwt/v4"
)

// Taken from https://cloud.google.com/confidential-computing/confidential-space/docs/connect-external-resources#pki-attestation-tokens
// ValidatePKIToken validates the PKI token returned from the attestation service is valid.
// Returns a valid jwt.Token or returns an error if invalid.
func ValidatePKIToken(storedRootCertificate *x509.Certificate, attestationToken string) (jwt.Token, error) {
	// IMPORTANT: The attestation token should be considered untrusted until the certificate chain and
	// the signature is verified.
	jwtHeaders, err := ExtractJWTHeaders(attestationToken)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("cannot extract JWTHeaders returned error: %w", err)
	}
	if jwtHeaders["alg"] != "RS256" {
		return jwt.Token{}, fmt.Errorf("cannot validate PKI TOKEN - got Alg: %v, want: %v", jwtHeaders["alg"], "RS256")
	}
	// Additional Check: Validate the ALG in the header matches the certificate SPKI.
	// https://datatracker.ietf.org/doc/html/rfc5280#section-4.1.2.7
	// This is included in golangs jwt.Parse function
	x5cHeaders, ok := jwtHeaders["x5c"].([]any)
	if !ok {
		return jwt.Token{}, errors.New("jwtHeaders[x5c] is not a slice")
	}
	certificates, err := ExtractCertificatesFromX5CHeader(x5cHeaders)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("cannot ExtractCertificatesFromX5CHeader: %w", err)
	}
	// Verify the leaf certificate signature algorithm is an RSA key
	if certificates.LeafCert.SignatureAlgorithm != x509.SHA256WithRSA {
		return jwt.Token{}, errors.New("leaf certificate signature algorithm is not SHA256WithRSA")
	}
	// Verify the leaf certificate public key algorithm is RSA
	if certificates.LeafCert.PublicKeyAlgorithm != x509.RSA {
		return jwt.Token{}, errors.New("leaf certificate public key algorithm is not RSA")
	}
	// Verify the storedRootCertificate is the same as the root certificate returned in the token.
	// storedRootCertificate is downloaded from the confidential computing well known endpoint
	// https://confidentialcomputing.googleapis.com/.well-known/attestation-pki-root
	err = CompareCertificates(storedRootCertificate, certificates.RootCert)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("failed to verify certificate chain: %w", err)
	}
	err = VerifyCertificateChain(certificates)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("verification certificate chain failed: %w", err)
	}
	keyFunc := func(token *jwt.Token) (any, error) {
		return certificates.LeafCert.PublicKey, nil
	}
	verifiedJWT, err := jwt.ParseWithClaims(attestationToken, &teetype.GoogleTeeClaims{}, keyFunc)
	if err != nil {
		return jwt.Token{}, err
	}
	return *verifiedJWT, nil
}

// ExtractJWTHeaders parses the JWT and returns the headers.
func ExtractJWTHeaders(token string) (map[string]any, error) {
	parser := &jwt.Parser{}
	// The claims returned from the token are unverified at this point
	// Do not use the claims until the algorithm, certificate chain verification and root certificate
	// comparison is successful
	unverifiedClaims := &jwt.MapClaims{}
	parsedToken, _, err := parser.ParseUnverified(token, unverifiedClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to parse claims token: %w", err)
	}
	return parsedToken.Header, nil
}

// PKICertificates contains the certificates extracted from the x5c header.
type PKICertificates struct {
	LeafCert         *x509.Certificate
	IntermediateCert *x509.Certificate
	RootCert         *x509.Certificate
}

// ExtractCertificatesFromX5CHeader extracts the certificates from the given x5c header.
func ExtractCertificatesFromX5CHeader(x5cHeaders []any) (PKICertificates, error) {
	if x5cHeaders == nil {
		return PKICertificates{}, fmt.Errorf("x5c header not set")
	}
	x5c := []string{}
	for _, header := range x5cHeaders {
		h, ok := header.(string)
		if !ok {
			return PKICertificates{}, fmt.Errorf("header %v is not a string", header)
		}
		x5c = append(x5c, h)
	}
	// The PKI token x5c header should have 3 certificates - leaf, intermediate and root
	const numberOfCertificates = 3
	if len(x5c) != numberOfCertificates {
		return PKICertificates{}, fmt.Errorf("incorrect number of certificates in x5c header, expected 3 certificates, but got %v", len(x5c))
	}
	leafCert, err := DecodeAndParseDERCertificate(x5c[0])
	if err != nil {
		return PKICertificates{}, fmt.Errorf("cannot parse leaf certificate: %w", err)
	}
	intermediateCert, err := DecodeAndParseDERCertificate(x5c[1])
	if err != nil {
		return PKICertificates{}, fmt.Errorf("cannot parse intermediate certificate: %w", err)
	}
	rootCert, err := DecodeAndParseDERCertificate(x5c[2])
	if err != nil {
		return PKICertificates{}, fmt.Errorf("cannot parse root certificate: %w", err)
	}
	certificates := PKICertificates{
		LeafCert:         leafCert,
		IntermediateCert: intermediateCert,
		RootCert:         rootCert,
	}
	return certificates, nil
}

// DecodeAndParseDERCertificate decodes the given DER certificate string and parses it into an x509 certificate.
func DecodeAndParseDERCertificate(certificate string) (*x509.Certificate, error) {
	bytes, err := base64.StdEncoding.DecodeString(certificate)
	if err != nil {
		return nil, fmt.Errorf("cannot decode base64 certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(bytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse certificate: %w", err)
	}
	return cert, nil
}

// VerifyCertificateChain verifies the certificate chain from leaf to root.
// It also checks that all certificate lifetimes are valid.
func VerifyCertificateChain(certificates PKICertificates) error {
	if !isCertificateLifetimeValid(certificates.LeafCert) {
		return fmt.Errorf("leaf certificate is not valid")
	}
	if !isCertificateLifetimeValid(certificates.IntermediateCert) {
		return fmt.Errorf("intermediate certificate is not valid")
	}
	interPool := x509.NewCertPool()
	interPool.AddCert(certificates.IntermediateCert)
	if !isCertificateLifetimeValid(certificates.RootCert) {
		return fmt.Errorf("root certificate is not valid")
	}
	rootPool := x509.NewCertPool()
	rootPool.AddCert(certificates.RootCert)
	_, err := certificates.LeafCert.Verify(x509.VerifyOptions{
		Intermediates: interPool,
		Roots:         rootPool,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		return fmt.Errorf("failed to verify certificate chain: %w", err)
	}
	return nil
}

func isCertificateLifetimeValid(certificate *x509.Certificate) bool {
	currentTime := time.Now().UTC()
	if currentTime.Before(certificate.NotBefore) {
		return false
	}
	if currentTime.After(certificate.NotAfter) {
		return false
	}

	return true
}

// CompareCertificates compares two certificate fingerprints.
func CompareCertificates(cert1, cert2 *x509.Certificate) error {
	fingerprint1 := sha256.Sum256(cert1.Raw)
	fingerprint2 := sha256.Sum256(cert2.Raw)
	if fingerprint1 != fingerprint2 {
		return fmt.Errorf("certificate fingerprint mismatch")
	}
	return nil
}

func ValidateClaims(claims *teetype.GoogleTeeClaims, teeInfoData teenodetype.TeeInfo, allowDebugMode bool) (teetype.StatusInfo, error) {
	var statusInfo teetype.StatusInfo
	if len(claims.EATNonce) != 1 {
		return teetype.StatusInfo{}, fmt.Errorf("expected exactly one EATNonce, got %d", len(claims.EATNonce))
	}
	// generate teeInfo hash
	teeInfoBytes, err := teeInfoData.Hash()
	if err != nil {
		return teetype.StatusInfo{}, fmt.Errorf("cannot create hash of teeInfo: %w", err)
	}
	// match with eat_nonce
	if claims.EATNonce[0] != hex.EncodeToString(teeInfoBytes) {
		return teetype.StatusInfo{}, fmt.Errorf("EATNonce does not match hash of teeInfo")
	}
	// Check if running in production. Allow debug mode only if ALLOW_TEE_DEBUG is enabled.
	if allowDebugMode {
		if claims.DebugStatus == "disabled-since-boot" {
			return teetype.StatusInfo{}, errors.New("production TEE not allowed when ALLOW_TEE_DEBUG=true")
		}
		// No check for supported attributes in debug mode
		statusInfo.Status = teetype.OK
	} else {
		// Non-debug mode
		if claims.DebugStatus != "disabled-since-boot" {
			return teetype.StatusInfo{}, errors.New("TEE is not running in production mode")
		}
		// Check Confidential Space image version
		if claims.SubMods.ConfidentialSpace.SupportAttributes == nil {
			return teetype.StatusInfo{}, errors.New("ConfidentialSpace component has no supported attributes")
		}
		foundIsStable := false
		for _, att := range claims.SubMods.ConfidentialSpace.SupportAttributes {
			if att == "STABLE" {
				foundIsStable = true
				break
			}
		}
		if !foundIsStable {
			statusInfo.Status = teetype.OBSOLETE
		} else {
			statusInfo.Status = teetype.OK
		}
	}
	// Check the OS is Confidential Space
	if claims.SWName != "CONFIDENTIAL_SPACE" {
		return teetype.StatusInfo{}, fmt.Errorf("SWName check failed: expected CONFIDENTIAL_SPACE, got %q", claims.SWName)
	}
	statusInfo.CodeHash, err = coreutil.HexStringToBytes32(strings.TrimPrefix(claims.SubMods.Container.ImageDigest, "sha256:"))
	if err != nil {
		return teetype.StatusInfo{}, fmt.Errorf("cannot convert container.image_digest %q to Bytes32: %w", claims.SubMods.Container.ImageDigest, err)
	}
	statusInfo.Platform, err = coreutil.StringToBytes32(claims.HWModel)
	if err != nil {
		return teetype.StatusInfo{}, fmt.Errorf("cannot convert HWModel %s to Bytes32: %w", claims.HWModel, err)
	}
	return statusInfo, nil
}
