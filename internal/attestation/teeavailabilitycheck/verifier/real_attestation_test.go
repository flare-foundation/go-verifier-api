package verifier_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// Manual/debug harness (not run in CI): drive the googlecloud verification with
// a real tee-node attestation response, to validate the cert chain, JWT
// signature, policy, and the teeInfo.Hash() -> eat_nonce binding against a live
// token. Useful when checking a new TEE deploy or tee-node/TeeInfo alignment.
//
// TestRealAttestationGoogleCloud mirrors the production DataVerification path
// (internal/attestation/teeavailabilitycheck/verifier/verifier.go): it parses a
// full tee-node TeeInfoResponse, derives the expected eat_nonce from
// teeInfo.Hash(), builds the same googlecloud.Policy, and runs
// ParseAndValidatePKIToken against the embedded Google root certificate.
//
// Tokens are time-limited (~1h) and signature checks hit the real Google cert
// chain, so the test is gated behind RUN_REAL_ATTESTATION=1 and must not run in
// CI. Provide the full tee-node response JSON (the object with teeInfo /
// machineData / attestation) via either env var:
//
//	RUN_REAL_ATTESTATION=1 REAL_ATTESTATION_RESPONSE_FILE=/tmp/resp.json \
//	  go test ./internal/attestation/teeavailabilitycheck/verifier/ \
//	  -run TestRealAttestationGoogleCloud -v
//
// or REAL_ATTESTATION_RESPONSE='{...}' inline. Optional overrides:
//   - TEE_AUDIENCE   (default https://sts.google.com)
//   - PROD_DBGSTAT=1 require dbgstat "disabled-since-boot" (production mode);
//     unset mirrors ALLOW_TEE_DEBUG=true and accepts debug TEEs.
func TestRealAttestationGoogleCloud(t *testing.T) {
	if os.Getenv("RUN_REAL_ATTESTATION") == "" {
		t.Skip("set RUN_REAL_ATTESTATION=1 to run against a real tee-node response")
	}

	raw := []byte(os.Getenv("REAL_ATTESTATION_RESPONSE"))
	if len(raw) == 0 {
		path := os.Getenv("REAL_ATTESTATION_RESPONSE_FILE")
		require.NotEmpty(t, path, "set REAL_ATTESTATION_RESPONSE or REAL_ATTESTATION_RESPONSE_FILE")
		b, err := os.ReadFile(path)
		require.NoError(t, err, "read REAL_ATTESTATION_RESPONSE_FILE")
		raw = b
	}

	var resp teenodetypes.TeeInfoResponse
	require.NoError(t, json.Unmarshal(raw, &resp), "unmarshal tee-node response")
	require.NotEmpty(t, resp.Attestation, "response has no attestation token")

	// Production binding: the policy's expected eat_nonce is hex(teeInfo.Hash()),
	// computed from the same response (verifier.go DataVerification).
	teeInfoHash, err := resp.TeeInfo.Hash()
	require.NoError(t, err, "hash teeInfo")

	// Mirror verifier.go's policy. Empty AllowedDebugStatuses == ALLOW_TEE_DEBUG=true
	// (skip dbgstat allowlist); PROD_DBGSTAT=1 enforces the production status.
	var allowedDebugStatuses []string
	if os.Getenv("PROD_DBGSTAT") != "" {
		allowedDebugStatuses = []string{"disabled-since-boot"}
	}
	audience := os.Getenv("TEE_AUDIENCE")
	if audience == "" {
		audience = config.DefaultTeeAudience
	}
	policy := googlecloud.Policy{
		Audience:             audience,
		EATNonce:             hex.EncodeToString(teeInfoHash),
		AllowedDebugStatuses: allowedDebugStatuses,
		Issuer:               googlecloud.ConfidentialSpaceIssuer,
	}

	// Diagnostic: show the verifier-computed nonce vs the token's eat_nonce.
	if _, tokClaims, derr := googlecloud.ParsePKITokenUnverified(resp.Attestation); derr == nil {
		t.Logf("computed eat_nonce (hex teeInfo.Hash) = %s", hex.EncodeToString(teeInfoHash))
		t.Logf("token    eat_nonce claim             = %v", tokClaims.EATNonce)
	}

	root, err := config.LoadGoogleRootCert()
	require.NoError(t, err, "load embedded Google root certificate")

	// nil CRLs: skip revocation checking (the verifier only checks CRLs when a
	// CRLCache is configured).
	_, claims, err := googlecloud.ParseAndValidatePKIToken(resp.Attestation, root, nil, nil, policy)
	if errors.Is(err, jwt.ErrTokenExpired) {
		t.Skipf("token cryptographically valid (cert chain + signature OK) but expired; fetch a fresh response: %v", err)
	}
	require.NoError(t, err, "ParseAndValidatePKIToken (production path) should accept the token")
	require.NotNil(t, claims)

	t.Logf("OK SWName=%s dbgstat=%s hwmodel=%s image_id=%s eat_nonce(teeInfo.Hash)=%s",
		claims.SWName, claims.DebugStatus, claims.HWModel,
		claims.SubMods.Container.ImageID, hex.EncodeToString(teeInfoHash))

	require.Equal(t, "CONFIDENTIAL_SPACE", claims.SWName)
}
