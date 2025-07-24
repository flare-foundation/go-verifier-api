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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/tee"
	apitypes "github.com/flare-foundation/go-verifier-api/internal/api/type"
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
		return jwt.Token{}, fmt.Errorf("cannot extract JWTHeaders returned error: %v", err)
	}
	if jwtHeaders["alg"] != "RS256" {
		return jwt.Token{}, fmt.Errorf(fmt.Sprintf("Cannot validate PKI TOKEN - got Alg: %v, want: %v", jwtHeaders["alg"], "RS256"), nil)
	}
	// Additional Check: Validate the ALG in the header matches the certificate SPKI.
	// https://datatracker.ietf.org/doc/html/rfc5280#section-4.1.2.7
	// This is included in golangs jwt.Parse function
	x5cHeaders := jwtHeaders["x5c"].([]any)
	certificates, err := ExtractCertificatesFromX5CHeader(x5cHeaders)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("cannot ExtractCertificatesFromX5CHeader: %v", err)
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
		return jwt.Token{}, fmt.Errorf("failed to verify certificate chain: %v", err)
	}
	err = VerifyCertificateChain(certificates)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("verification certificate chain failed: %v", err)
	}
	keyFunc := func(token *jwt.Token) (any, error) {
		return certificates.LeafCert.PublicKey, nil
	}
	verifiedJWT, err := jwt.Parse(attestationToken, keyFunc)
	return *verifiedJWT, fmt.Errorf("jwt.Parse error: %v", err)
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
		return nil, fmt.Errorf("failed to parse claims token: %v", err)
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
		x5c = append(x5c, header.(string))
	}
	// The PKI token x5c header should have 3 certificates - leaf, intermediate and root
	if len(x5c) != 3 {
		return PKICertificates{}, fmt.Errorf("incorrect number of certificates in x5c header, expected 3 certificates, but got %v", len(x5c))
	}
	leafCert, err := DecodeAndParseDERCertificate(x5c[0])
	if err != nil {
		return PKICertificates{}, fmt.Errorf("cannot parse leaf certificate: %v", err)
	}
	intermediateCert, err := DecodeAndParseDERCertificate(x5c[1])
	if err != nil {
		return PKICertificates{}, fmt.Errorf("cannot parse intermediate certificate: %v", err)
	}
	rootCert, err := DecodeAndParseDERCertificate(x5c[2])
	if err != nil {
		return PKICertificates{}, fmt.Errorf("cannot parse root certificate: %v", err)
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
	bytes, _ := base64.StdEncoding.DecodeString(certificate)
	cert, err := x509.ParseCertificate(bytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse certificate: %v", err)
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
		return fmt.Errorf("failed to verify certificate chain: %v", err)
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

type GoogleTeeClaims struct {
	HWModel     string     `json:"hwmodel"` //"hwmodel": "GCP_INTEL_TDX"
	SWName      string     `json:"swname"`  //"swname": "CONFIDENTIAL_SPACE"
	SecBoot     bool       `json:"secboot"`
	EATNonce    []string   `json:"eat_nonce"`
	SubModules  SubModules `json:"submods"`
	DebugStatus string     `json:"dbgstat"` //"dbgstat": "enabled"
	jwt.StandardClaims
}

type SubModules struct {
	ConfidentialSpace ConfidentialSpaceInfo `json:"confidential_space"`
	Container         Container             `json:"container"`
}

type ConfidentialSpaceInfo struct {
	SupportAttributes []string `json:"support_attributes"`
}

type Container struct {
	ImageDigest string `json:"image_digest"` //"image_digest": "sha256:0f5455255ce543c2fa319153577e2ad75d7f8ea698df1cab1a8c782b391b6354",
	ImageId     string `json:"image_id"`     //"image_id": "sha256:ec5873e29dd512750dfd21250db6243f106bbf82203e91ae33af94b234eee153"
}

type StatusInfo struct {
	CodeHash common.Hash
	Platform common.Hash
	Status   apitypes.AvailabilityCheckStatus
}

func ValidateClaims(token jwt.Token, infoData tee.TeeStructsAttestation) (StatusInfo, error) {
	var statusInfo StatusInfo
	if !token.Valid { // probably unnecessary
		return StatusInfo{}, fmt.Errorf("attestation token is invalid: %v", token)
	}
	claims, ok := token.Claims.(*GoogleTeeClaims)
	if !ok {
		return StatusInfo{}, errors.New("cannot parse claims")
	}
	// generate teeInfo hash
	teeInfoHash, err := TeeInfoHash(infoData)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot create hash of teeInfo: %v", err)
	}
	// match with eat_nonce - TODO check if it is really string array or just string
	if claims.EATNonce[0] != teeInfoHash {
		return StatusInfo{}, errors.New("eat_nonce does not match")
	}
	// Check if running in production
	if claims.DebugStatus != "disabled-since-boot" {
		return StatusInfo{}, errors.New("not running in production mode")
	}
	// Check the OS is Confidential Space
	if claims.SWName != "CONFIDENTIAL_SPACE" {
		return StatusInfo{}, errors.New("not running in CONFIDENTIAL_SPACE")
	}
	// Check Confidential Space image version
	foundIsStable := false
	for _, att := range claims.SubModules.ConfidentialSpace.SupportAttributes {
		if att == "STABLE" {
			foundIsStable = true
			break
		}
	}
	if !foundIsStable {
		statusInfo.Status = apitypes.OBSOLETE
	} else {
		statusInfo.Status = apitypes.OK
	}
	statusInfo.CodeHash, err = hexStringToBytes32(strings.TrimPrefix(claims.SubModules.Container.ImageDigest, "sha256:"))
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot retrieve hash of container.image_digest: %v", err)
	}
	statusInfo.Platform, err = hexStringToBytes32(strings.TrimPrefix(claims.HWModel, "sha256:")) //TODO - fix need to decide about the type first
	if err != nil {
		return StatusInfo{}, fmt.Errorf("cannot retrieve hash of hwmodel: %v", err)
	}
	return statusInfo, nil
}

func hexStringToBytes32(hexStr string) (common.Hash, error) {
	var b32 common.Hash
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return b32, fmt.Errorf("invalid hex string: %w", err)
	}
	if len(b) != 32 {
		return b32, fmt.Errorf("expected 32 bytes but got %d bytes", len(b))
	}
	copy(b32[:], b)
	return b32, nil
}

// Copied from https://gitlab.com/flarenetwork/tee/tee-node/-/blob/brezTilna/internal/attestation/attestation.go#L55
func TeeInfoHash(teeInfo tee.TeeStructsAttestation) (string, error) {
	encoded, err := structs.Encode(tee.StructArg[tee.Attestation], teeInfo)
	if err != nil {
		return "", fmt.Errorf("cannot create teeInfoHash: %v", err)
	}
	hashBytes := crypto.Keccak256(encoded)
	hashString := hex.EncodeToString(hashBytes[:])
	return hashString, nil
}
