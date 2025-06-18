package verification

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt"

	"gitlab.com/urskak/tee-availability-check/pkg/tee_availability_check/types"
)

// Taken from https://cloud.google.com/confidential-computing/confidential-space/docs/connect-external-resources#pki-attestation-tokens

// This code is an example of how to validate a PKI token. This library is not an official library,
// nor is it endorsed by Google.

// ValidatePKIToken validates the PKI token returned from the attestation service is valid.
// Returns a valid jwt.Token or returns an error if invalid.
func ValidatePKIToken(storedRootCertificate *x509.Certificate, attestationToken string) (jwt.Token, error) {
	// IMPORTANT: The attestation token should be considered untrusted until the certificate chain and
	// the signature is verified.

	jwtHeaders, err := ExtractJWTHeaders(attestationToken)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("ExtractJWTHeaders(token) returned error: %v", err)
	}

	if jwtHeaders["alg"] != "RS256" {
		return jwt.Token{}, fmt.Errorf("ValidatePKIToken(string, *attestpb.Attestation, *v1mainpb.VerifyAttestationRequest) - got Alg: %v, want: %v", jwtHeaders["alg"], "RS256")
	}

	// Additional Check: Validate the ALG in the header matches the certificate SPKI.
	// https://datatracker.ietf.org/doc/html/rfc5280#section-4.1.2.7
	// This is included in golangs jwt.Parse function

	x5cHeaders := jwtHeaders["x5c"].([]any)
	certificates, err := ExtractCertificatesFromX5CHeader(x5cHeaders)
	if err != nil {
		return jwt.Token{}, fmt.Errorf("ExtractCertificatesFromX5CHeader(x5cHeaders) returned error: %v", err)
	}

	// Verify the leaf certificate signature algorithm is an RSA key
	if certificates.LeafCert.SignatureAlgorithm != x509.SHA256WithRSA {
		return jwt.Token{}, fmt.Errorf("leaf certificate signature algorithm is not SHA256WithRSA")
	}

	// Verify the leaf certificate public key algorithm is RSA
	if certificates.LeafCert.PublicKeyAlgorithm != x509.RSA {
		return jwt.Token{}, fmt.Errorf("leaf certificate public key algorithm is not RSA")
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
		return jwt.Token{}, fmt.Errorf("VerifyCertificateChain(string, *attestpb.Attestation, *v1mainpb.VerifyAttestationRequest) - error verifying x5c chain: %v", err)
	}

	keyFunc := func(token *jwt.Token) (any, error) {
		return certificates.LeafCert.PublicKey, nil
	}

	verifiedJWT, err := jwt.Parse(attestationToken, keyFunc)
	return *verifiedJWT, err
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
		return PKICertificates{}, fmt.Errorf("VerifyAttestation(string, *attestpb.Attestation, *v1mainpb.VerifyAttestationRequest) - x5c header not set")
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

// DecodeAndParsePEMCertificate decodes the given PEM certificate string and parses it into an x509 certificate.
func DecodeAndParsePEMCertificate(certificate string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		return nil, fmt.Errorf("cannot decode certificate: invalid PEM format")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
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
	currentTime := time.Now()
	// check the current time should be after the certificate NotBefore time
	if currentTime.Before(certificate.NotBefore) {
		return false
	}
	// check the current time should be before the certificate NotAfter time
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
	HWModel     string     `json:"hwmodel"`
	SWName      string     `json:"swname"`
	SecBoot     bool       `json:"secboot"`
	EATNonce    []string   `json:"eat_nonce"`
	SubModules  SubModules `json:"submods"`
	DebugStatus string     `json:"dbgstat"`
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
	ImageDigest string `json:"image_digest"`
	ImageId     string `json:"image_id"`
}

type StatusInfo struct {
	CodeHash [32]byte
	Platform [32]byte
	Status   types.AvailabilityCheckStatus
}

func ValidateClaims(token jwt.Token, infoData types.ProxyInfoData) (StatusInfo, error) {
	var statusInfo StatusInfo
	if !token.Valid {
		return StatusInfo{}, errors.New("token not valid")
	}
	claims, ok := token.Claims.(*GoogleTeeClaims)
	if !ok {
		return StatusInfo{}, errors.New("cannot parse claims")
	}
	// TODO validate eat_nonce with data from tee proxy using infoData
	teeId := crypto.PubkeyToAddress(infoData.PublicKey)
	eatNonce := crypto.Keccak256([]byte(infoData.Challenge), teeId[:], crypto.Keccak256([]byte{byte(infoData.Status)}), infoData.InitialSigningPolicyHash[:], infoData.LastSigningPolicyHash[:], infoData.TeeGovernanceHash[:])
	if claims.EATNonce[0] != common.Bytes2Hex(eatNonce) {
		return StatusInfo{}, errors.New("EATNonce does not match")
	}
	// Check if running in production
	if claims.DebugStatus != "disabled-since-boot" {
		return StatusInfo{}, errors.New("not in production mode")
	}
	// Check the OS is Confidential Space
	if claims.SWName != "CONFIDENTIAL_SPACE" {
		return StatusInfo{}, errors.New("not running CONFIDENTIAL_SPACE")
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
		statusInfo.Status = types.OBSOLETE
	} else {
		statusInfo.Status = types.OK
	}
	statusInfo.CodeHash = common.BytesToHash([]byte(claims.SubModules.Container.ImageDigest))
	statusInfo.Platform = common.BytesToHash([]byte(claims.HWModel))

	return statusInfo, nil
}

func LoadRootCert() (*x509.Certificate, error) {
	rootCertBytes, err := os.ReadFile("../google_confidential_space_root.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to read root certificate: %w", err)
	}

	cert, err := DecodeAndParsePEMCertificate(string(rootCertBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse root certificate: %w", err)
	}

	return cert, nil
}
