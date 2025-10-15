package verifier_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	teenodetype "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestValidateClaims(t *testing.T) {
	teeInfoData := teenodetype.TeeInfo{
		Challenge: common.HexToHash("0x1"),
	}
	teeInfoHash, err := teeInfoData.Hash()
	require.NoError(t, err)
	baseClaims := &teetype.GoogleTeeClaims{
		HWModel:     "GCP_INTEL_TDX",
		SWName:      "CONFIDENTIAL_SPACE",
		EATNonce:    []string{hex.EncodeToString(teeInfoHash)},
		DebugStatus: "disabled-since-boot",
		SubMods: teetype.SubModules{
			ConfidentialSpace: teetype.ConfidentialSpaceInfo{
				SupportAttributes: []string{"STABLE"},
			},
			Container: teetype.Container{
				ImageDigest: "sha256:0f5455255ce543c2fa319153577e2ad75d7f8ea698df1cab1a8c782b391b6354",
			},
		},
	}

	t.Run("valid", func(t *testing.T) {
		_, err := verifier.ValidateClaims(baseClaims, teeInfoData, false)
		require.NoError(t, err)
	})
	t.Run("valid in debug mode", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.DebugStatus = "enabled"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, true)
		require.NoError(t, err)
	})
	t.Run("production TEE not allowed in debug mode", func(t *testing.T) {
		_, err := verifier.ValidateClaims(baseClaims, teeInfoData, true)
		require.ErrorContains(t, err, "production TEE not allowed when ALLOW_TEE_DEBUG=true")
	})
	t.Run("debug TEE not allowed in production mode", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.DebugStatus = "enabled"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "not running in production mode")
	})
	t.Run("expect one EATNonce", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.EATNonce = []string{}
		val, err := verifier.ValidateClaims(&modClaims, teenodetype.TeeInfo{}, true)
		require.Equal(t, teetype.StatusInfo{}, val)
		require.ErrorContains(t, err, "expected exactly one EATNonce, got 0")
	})
	t.Run("EATNonce does not match", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.EATNonce = []string{"123"}
		val, err := verifier.ValidateClaims(&modClaims, teenodetype.TeeInfo{}, true)
		require.Equal(t, teetype.StatusInfo{}, val)
		require.ErrorContains(t, err, "EATNonce does not match hash of teeInfo")
	})
	t.Run("no supported attributes", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = nil
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "ConfidentialSpace component has no supported attributes")
	})
	t.Run("no supported attributes", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.ConfidentialSpace.SupportAttributes = []string{}
		val, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.NoError(t, err)
		require.Equal(t, teetype.OBSOLETE, val.Status)
	})
	t.Run("no confidential space", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SWName = "CONF"
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "SWName check failed: expected CONFIDENTIAL_SPACE")
	})
	t.Run("cannot retrieve hash of hwmodel", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.HWModel = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "cannot convert HWMode")
	})
	t.Run("cannot retrieve hash of container.image_digest", func(t *testing.T) {
		modClaims := *baseClaims
		modClaims.SubMods.Container.ImageDigest = "0x" + strings.Repeat("ff", 33)
		_, err := verifier.ValidateClaims(&modClaims, teeInfoData, false)
		require.ErrorContains(t, err, "cannot convert container.image_digest")
	})
}

func TestCompareCertificates(t *testing.T) {
	cert1 := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	cert2 := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	require.NoError(t, verifier.CompareCertificates(cert1, cert1))
	require.ErrorContains(t, verifier.CompareCertificates(cert1, cert2), "certificate fingerprint mismatch")
}

func TestExtractCertificatesFromX5CHeader(t *testing.T) {
	cert := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certPEM := base64.StdEncoding.EncodeToString(cert.Raw)

	t.Run("success", func(t *testing.T) {
		x5c := []any{certPEM, certPEM, certPEM}
		certs, err := verifier.ExtractCertificatesFromX5CHeader(x5c)
		require.NoError(t, err)
		require.NotNil(t, certs.LeafCert)
		require.NotNil(t, certs.IntermediateCert)
		require.NotNil(t, certs.RootCert)
	})
	t.Run("wrong type", func(t *testing.T) {
		certs, err := verifier.ExtractCertificatesFromX5CHeader([]any{123, 456, 789})
		require.Equal(t, verifier.PKICertificates{}, certs)
		require.ErrorContains(t, err, "is not a string")
	})
	t.Run("wrong number of certs", func(t *testing.T) {
		certs, err := verifier.ExtractCertificatesFromX5CHeader([]any{certPEM})
		require.Equal(t, verifier.PKICertificates{}, certs)
		require.ErrorContains(t, err, "incorrect number of certificates")
	})
	t.Run("invalid leaf base64", func(t *testing.T) {
		certs, err := verifier.ExtractCertificatesFromX5CHeader([]any{"!!!", "!!!", "!!!"})
		require.Equal(t, verifier.PKICertificates{}, certs)
		require.ErrorContains(t, err, "cannot parse leaf certificate: cannot decode base64 certificate")
	})
	t.Run("invalid intermediate base64", func(t *testing.T) {
		certs, err := verifier.ExtractCertificatesFromX5CHeader([]any{certPEM, "!!!", "!!!"})
		require.Equal(t, verifier.PKICertificates{}, certs)
		require.ErrorContains(t, err, "cannot parse intermediate certificate: cannot decode base64 certificate")
	})
	t.Run("invalid root base64", func(t *testing.T) {
		invalidDER := base64.StdEncoding.EncodeToString([]byte("not a real certificate"))
		certs, err := verifier.ExtractCertificatesFromX5CHeader([]any{certPEM, certPEM, invalidDER})
		require.Equal(t, verifier.PKICertificates{}, certs)
		require.ErrorContains(t, err, "cannot parse root certificate: cannot parse certificate")
	})
	t.Run("nil x5cHeaders", func(t *testing.T) {
		certs, err := verifier.ExtractCertificatesFromX5CHeader(nil)
		require.Equal(t, verifier.PKICertificates{}, certs)
		require.ErrorContains(t, err, "x5c header not set")
	})
}

func TestVerifyCertificateChain(t *testing.T) {
	cert := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certs := verifier.PKICertificates{
		LeafCert:         cert,
		IntermediateCert: cert,
		RootCert:         cert,
	}
	require.NoError(t, verifier.VerifyCertificateChain(certs))
	expired := testhelper.GenerateTestCertificate(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	notValidYet := testhelper.GenerateTestCertificate(t, time.Now().Add(2*time.Hour), time.Now().Add(3*time.Hour))

	t.Run("expired leaf", func(t *testing.T) {
		leafCerts := certs
		leafCerts.LeafCert = notValidYet
		err := verifier.VerifyCertificateChain(leafCerts)
		require.ErrorContains(t, err, "leaf certificate is not valid")
	})
	t.Run("expired intermediate", func(t *testing.T) {
		intermediateCerts := certs
		intermediateCerts.IntermediateCert = expired
		err := verifier.VerifyCertificateChain(intermediateCerts)
		require.ErrorContains(t, err, "intermediate certificate is not valid")
	})
	t.Run("expired root", func(t *testing.T) {
		rootCerts := certs
		rootCerts.RootCert = expired
		err := verifier.VerifyCertificateChain(rootCerts)
		require.ErrorContains(t, err, "root certificate is not valid")
	})
	t.Run("valid", func(t *testing.T) {
		require.NoError(t, verifier.VerifyCertificateChain(certs))
	})
	t.Run("cannot verify certificate chain", func(t *testing.T) {
		leaf := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
		intermediate := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
		root := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
		certs := verifier.PKICertificates{
			LeafCert:         leaf,
			IntermediateCert: intermediate,
			RootCert:         root,
		}
		err := verifier.VerifyCertificateChain(certs)
		require.ErrorContains(t, err, "failed to verify certificate chain")
	})
}

func TestValidatePKIToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cert := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	t.Run("extract JWTHeaders fails", func(t *testing.T) {
		badToken := "this-is-not-a-jwt"
		cert := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

		val, err := verifier.ValidatePKIToken(cert, badToken)
		require.Equal(t, jwt.Token{}, val)
		require.ErrorContains(t, err, "cannot extract JWTHeaders")
	})
	t.Run("wrong alg header", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"foo": "bar"}) // HS256 instead of RS256
		signedToken, _ := token.SignedString([]byte("secret"))

		val, err := verifier.ValidatePKIToken(cert, signedToken)
		require.Equal(t, jwt.Token{}, val)
		require.ErrorContains(t, err, "cannot validate PKI TOKEN - got Alg")
	})
	t.Run("x5c not a slice", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"foo": "bar"})
		token.Header["x5c"] = "not-a-slice" // intentionally wrong
		signedToken, _ := token.SignedString(privKey)

		val, err := verifier.ValidatePKIToken(cert, signedToken)
		require.Equal(t, jwt.Token{}, val)
		require.ErrorContains(t, err, "jwtHeaders[x5c] is not a slice")
	})
	t.Run("valid token fails on chain", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"foo": "bar"})
		token.Header["x5c"] = []string{"invalid-cert", "invalid-cert", "invalid-cert"}
		signedToken, err := token.SignedString(privKey)
		require.NoError(t, err)

		val, err := verifier.ValidatePKIToken(cert, signedToken)
		require.Equal(t, jwt.Token{}, val)
		require.Error(t, err) // will fail in chain verification, just confirms flow works
	})
}

func TestExtractJWTHeaders(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"foo": "bar"})
	signed, _ := token.SignedString([]byte("secret"))

	headers, err := verifier.ExtractJWTHeaders(signed)
	require.NoError(t, err)
	require.Equal(t, "HS256", headers["alg"])

	val, err := verifier.ExtractJWTHeaders("invalid.token")
	require.Nil(t, val)
	require.ErrorContains(t, err, "failed to parse claims token: token contains an invalid number of segments")
}
