package verifier_test

import (
	"context"
	cr "crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	csigning "github.com/flare-foundation/go-flare-common/pkg/signing"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-flare-common/pkg/tee/op"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/teeavailabilitycheck/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFetchSigningPolicyHashFromChain(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{
			RelayCaller: mockRelay,
		}
		expectedHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
		var hashBytes [32]byte
		copy(hashBytes[:], expectedHash.Bytes())
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(42)).Return(hashBytes, nil)
		hash, _, err := v.FetchSigningPolicyHashFromChain(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, expectedHash, hash)
		mockRelay.AssertExpectations(t)
	})
	t.Run("failure", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{
			RelayCaller: mockRelay,
		}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(99)).Return([32]byte{}, errors.New("rpc error"))
		_, _, err := v.FetchSigningPolicyHashFromChain(context.Background(), 99)
		require.ErrorContains(t, err, "ToSigningPolicyHash: unknown error")
		mockRelay.AssertExpectations(t)
	})
}

func TestFetchSigningPolicyHashFromChainWithRetry(t *testing.T) {
	expectedHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	var hashBytes [32]byte
	copy(hashBytes[:], expectedHash.Bytes())
	maxAttempts := 2
	delay := 150 * time.Millisecond

	t.Run("success on first attempt", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(42)).Return(hashBytes, nil)
		hash, state, err := v.FetchSigningPolicyHashFromChainWithRetry(context.Background(), 42, maxAttempts, delay)
		require.NoError(t, err)
		require.Equal(t, verifiertypes.TeeSampleValid, state)
		require.Equal(t, expectedHash, hash)
		mockRelay.AssertExpectations(t)
	})
	t.Run("succeeds after retry", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		callCount := 0
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(43)).
			Return([32]byte{}, nil).
			Run(func(args mock.Arguments) {
				callCount++
				if callCount == 1 {
					mockRelay.ExpectedCalls[0].ReturnArguments = mock.Arguments{[32]byte{}, errors.New("rpc error")}
				} else {
					mockRelay.ExpectedCalls[0].ReturnArguments = mock.Arguments{hashBytes, nil}
				}
			})
		hash, state, err := v.FetchSigningPolicyHashFromChainWithRetry(context.Background(), 43, maxAttempts, delay)
		require.NoError(t, err)
		require.Equal(t, verifiertypes.TeeSampleValid, state)
		require.Equal(t, expectedHash, hash)
		require.GreaterOrEqual(t, callCount, 2, "should retry at least once")
		mockRelay.AssertExpectations(t)
	})
	t.Run("fails after all retries", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(99)).Return([32]byte{}, errors.New("rpc error"))
		hash, state, err := v.FetchSigningPolicyHashFromChainWithRetry(context.Background(), 99, maxAttempts, delay)
		require.ErrorContains(t, err, "fetchSigningPolicyHashFromChainWithRetry failed after 2 attempts: ToSigningPolicyHash: unknown error")
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		require.Equal(t, common.Hash{}, hash)
		mockRelay.AssertExpectations(t)
	})
}

func TestCheckSigningPolicies(t *testing.T) {
	expectedInitialHash := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	expectedLastHash := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
	var initialBytes, lastBytes [32]byte
	copy(initialBytes[:], expectedInitialHash.Bytes())
	copy(lastBytes[:], expectedLastHash.Bytes())

	baseTEEInfo := teenodetypes.TeeInfo{
		InitialSigningPolicyID:   1,
		InitialSigningPolicyHash: expectedInitialHash,
		LastSigningPolicyID:      2,
		LastSigningPolicyHash:    expectedLastHash,
	}
	t.Run("success", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), baseTEEInfo)
		require.NoError(t, err)
		require.Equal(t, verifiertypes.TeeSampleValid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("initial hash mismatch", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		modTEEInfo := baseTEEInfo
		modTEEInfo.InitialSigningPolicyHash = common.HexToHash("0xdeadbeef")
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), modTEEInfo)
		require.ErrorContains(t, err, "failed to validate initial signing policy hash")
		require.Equal(t, verifiertypes.TeeSampleInvalid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("last hash mismatch", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		modTEEInfo := baseTEEInfo
		modTEEInfo.LastSigningPolicyHash = common.HexToHash("0xdeadbeef")
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), modTEEInfo)
		require.ErrorContains(t, err, "failed to validate last signing policy hash")
		require.Equal(t, verifiertypes.TeeSampleInvalid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("fail to retrieve initial hash", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return([32]byte{}, errors.New("rpc error"))
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), baseTEEInfo)
		require.ErrorContains(t, err, "cannot retrieve initial signing policy hash for ID 1")
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("fail to retrieve last hash", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return([32]byte{}, errors.New("rpc error"))
		state, err := v.CheckSigningPolicies(context.Background(), baseTEEInfo)
		require.ErrorContains(t, err, "cannot retrieve last signing policy hash for ID 2")
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
		mockRelay.AssertExpectations(t)
	})
}

func makeChallengeResultServer(t *testing.T, resp teenodetypes.ActionResponse) *httptest.Server {
	t.Helper()
	handler := http.NewServeMux()
	handler.HandleFunc("/action/result/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(handler)
}

func TestFetchTEEChallengeResult(t *testing.T) {
	ctx := context.Background()
	challengeID := common.HexToHash("0x123")
	t.Run("success", func(t *testing.T) {
		validJSON := `{"teeInfo":{"InitialSigningPolicyID":1}}`
		data := hexutil.Bytes([]byte(validJSON))

		privKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		address := crypto.PubkeyToAddress(privKey.PublicKey)
		actionResult := teenodetypes.ActionResult{Data: data}
		// validJSON carries no chainId, so teeInfo.TeeInfo.ChainID is 0 on the recovery side.
		signHash, err := csigning.NewPayload(csigning.ProxyActionResult, 0, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		signature, err := crypto.Sign(accounts.TextHash(signHash[:]), privKey)
		require.NoError(t, err)

		fullResp := teenodetypes.ActionResponse{
			Result:         actionResult,
			ProxySignature: signature,
		}
		server := makeChallengeResultServer(t, fullResp)
		defer server.Close()
		actionResp, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.NoError(t, err)
		require.NotEqual(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, address, signer)
		// Full ActionResponse must be returned (signature and metadata propagated).
		require.Equal(t, fullResp.ProxySignature, actionResp.ProxySignature)
		require.Equal(t, fullResp.Result.Data, actionResp.Result.Data)
	})
	t.Run("signature bound to a different chainID recovers a foreign signer", func(t *testing.T) {
		// The proxy signs over DomainHash(PROXY_ACTION_RESULT, 1, …) but the
		// transmitted TeeInfo claims chainId 2. Recovery rebuilds the preimage
		// from the transmitted chainId, so it yields an address other than the
		// true proxy key — Verify then rejects it on the dataSigner != TeeProxyId
		// check. This is the cross-chain replay protection the binding provides.
		const signedChainID, claimedChainID uint64 = 1, 2
		validJSON := `{"teeInfo":{"chainId":2}}`
		data := hexutil.Bytes([]byte(validJSON))

		privKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		address := crypto.PubkeyToAddress(privKey.PublicKey)
		actionResult := teenodetypes.ActionResult{Data: data}
		signHash, err := csigning.NewPayload(csigning.ProxyActionResult, signedChainID, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		signature, err := crypto.Sign(accounts.TextHash(signHash[:]), privKey)
		require.NoError(t, err)

		server := makeChallengeResultServer(t, teenodetypes.ActionResponse{
			Result:         actionResult,
			ProxySignature: signature,
		})
		defer server.Close()
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.NoError(t, err)
		require.Equal(t, claimedChainID, teeInfo.TeeInfo.ChainID)
		// Recovery succeeds but returns a foreign address, never the true signer.
		require.NotEqual(t, address, signer)
	})
	t.Run("fetch error", func(t *testing.T) {
		handler := http.NewServeMux()
		handler.HandleFunc("/action/result/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})
		server := httptest.NewServer(handler)
		defer server.Close()
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.Error(t, err)
	})
	t.Run("empty data", func(t *testing.T) {
		server := makeChallengeResultServer(t, teenodetypes.ActionResponse{
			Result: teenodetypes.ActionResult{Data: hexutil.Bytes{}},
		})
		defer server.Close()
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "TEE challenge result data is empty")
	})
	t.Run("invalid JSON data", func(t *testing.T) {
		server := makeChallengeResultServer(t, teenodetypes.ActionResponse{
			Result: teenodetypes.ActionResult{Data: hexutil.Bytes([]byte("not-json"))},
		})
		defer server.Close()
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "TEE challenge result data is not valid JSON")
	})
	t.Run("invalid JSON data is truncated in error", func(t *testing.T) {
		// Build a non-JSON blob longer than 128 bytes to exercise the preview truncation path.
		large := make([]byte, 512)
		for i := range large {
			large[i] = 'x'
		}
		server := makeChallengeResultServer(t, teenodetypes.ActionResponse{
			Result: teenodetypes.ActionResult{Data: hexutil.Bytes(large)},
		})
		defer server.Close()
		_, _, _, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.ErrorContains(t, err, "TEE challenge result data is not valid JSON")
		require.ErrorContains(t, err, "len=512")
		// Preview must exist but must not carry the full 512 bytes verbatim.
		require.NotContains(t, err.Error(), string(large))
	})
	t.Run("unmarshal error", func(t *testing.T) {
		badJSON := `{"teeInfo":"this-should-be-an-object-not-a-string"}`
		server := makeChallengeResultServer(t, teenodetypes.ActionResponse{
			Result: teenodetypes.ActionResult{Data: hexutil.Bytes([]byte(badJSON))},
		})
		defer server.Close()
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "unmarshal TEE result")
	})
	t.Run("recover signer error", func(t *testing.T) {
		validJSON := `{"teeInfo":{"InitialSigningPolicyID":1}}`
		server := makeChallengeResultServer(t, teenodetypes.ActionResponse{
			Result:         teenodetypes.ActionResult{Data: hexutil.Bytes([]byte(validJSON))},
			ProxySignature: []byte("invalid-signature"),
		})
		defer server.Close()
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL, challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "recover signer")
	})
	t.Run("blocks private IP in strict mode", func(t *testing.T) {
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, "http://127.0.0.1", challengeID, false)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})
	t.Run("blocks dangerous IP in allow-private mode", func(t *testing.T) {
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, "http://169.254.169.254", challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})
	t.Run("invalid base URL", func(t *testing.T) {
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, "http://", challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "URL host is required")
	})
	t.Run("unparseable base URL fails path composition", func(t *testing.T) {
		// An invalid percent-escape makes url.JoinPath reject the base URL, so the
		// request URL can never be composed (covers the JoinPath error branch).
		_, teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, "http://%xx", challengeID, true)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "invalid proxy URL")
	})
	t.Run("base URL with path prefix and trailing slash", func(t *testing.T) {
		// Regression for the url.JoinPath composition: a baseURL carrying a path
		// prefix must be preserved and a trailing slash must not produce a double
		// slash. The handler is mounted under the prefix and the request path is
		// asserted exactly.
		validJSON := `{"teeInfo":{"InitialSigningPolicyID":1}}`
		data := hexutil.Bytes([]byte(validJSON))

		privKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		address := crypto.PubkeyToAddress(privKey.PublicKey)
		ethHash := accounts.TextHash(crypto.Keccak256(data))
		signature, err := crypto.Sign(ethHash, privKey)
		require.NoError(t, err)
		resp := teenodetypes.ActionResponse{
			Result:         teenodetypes.ActionResult{Data: data},
			ProxySignature: signature,
		}

		var gotPath string
		handler := http.NewServeMux()
		handler.HandleFunc("/proxy/action/result/", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		_, _, signer, err := verifier.FetchTEEChallengeResult(ctx, server.URL+"/proxy/", challengeID, true)
		require.NoError(t, err)
		require.Equal(t, address, signer)
		wantPath := "/proxy/action/result/" + hex.EncodeToString(challengeID.Bytes())
		require.Equal(t, wantPath, gotPath)
	})
}

// testAudience is an arbitrary non-empty value satisfying Policy.Audience.
const testAudience = "test-audience"

// testImageHash matches the sha256 image_id used in the test claims below.
var testImageHash = common.HexToHash("0x194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2")

// allowedImageIDsForTest is the AllowedImageIDs map used across tests that
// must satisfy the Confidential Space Policy.
func allowedImageIDsForTest() map[common.Hash]struct{} {
	return map[common.Hash]struct{}{testImageHash: {}}
}

// validTestClaims builds GoogleTeeClaims that pass the Policy enforced by
// ParseAndValidatePKIToken: matching audience and issuer, secboot enabled,
// production dbgstat, an image_id in the test allowlist, and an eat_nonce
// containing the supplied value.
func validTestClaims(eatNonce string) *googlecloud.GoogleTeeClaims {
	return &googlecloud.GoogleTeeClaims{
		HWModel:     "TEST_PLATFORM",
		SWName:      "CONFIDENTIAL_SPACE",
		SecBoot:     true,
		EATNonce:    []string{eatNonce},
		DebugStatus: "disabled-since-boot",
		SubMods: googlecloud.SubMods{
			ConfidentialSpace: googlecloud.ConfidentialSpaceInfo{
				SupportAttributes: []string{"STABLE"},
			},
			Container: googlecloud.Container{
				ImageID: "sha256:" + testImageHash.Hex()[2:],
			},
		},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    googlecloud.ConfidentialSpaceIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func TestDataVerification(t *testing.T) {
	rootCert, leafKey, x5c := generateTestCertificateChain(t)
	challengeHash := common.HexToHash("123")
	t.Run("MagicPass bypass returns test values", func(t *testing.T) {
		v := &verifier.TeeVerifier{Cfg: &config.TeeAvailabilityCheckConfig{}}
		teeID := common.HexToAddress("0x1234")
		resp := teenodetypes.TeeInfoResponse{
			Attestation: "magic_pass",
		}
		res, err := v.DataVerification(context.Background(), resp, teeID)
		require.NoError(t, err)
		require.Equal(t, verifier.E2ETestCodeHash, res.CodeHash)
		require.Equal(t, verifier.E2ETestPlatform, res.Platform)
	})
	t.Run("DisableAttestationCheckE2E", func(t *testing.T) {
		v := &verifier.TeeVerifier{Cfg: &config.TeeAvailabilityCheckConfig{DisableAttestationCheckE2E: true}}
		res, err := v.DataVerification(context.Background(), teenodetypes.TeeInfoResponse{}, common.Address{})
		require.NoError(t, err)
		require.Equal(t, verifier.E2ETestCodeHash, res.CodeHash)
		require.Equal(t, verifier.E2ETestPlatform, res.Platform)
	})
	t.Run("success", func(t *testing.T) {
		teeInfoResponse, privTEEKey := helpers.TeeInfoResponse(t, challengeHash)
		teeInfoHash, err := teeInfoResponse.TeeInfo.Hash()
		require.NoError(t, err)
		baseClaims := validTestClaims(hex.EncodeToString(teeInfoHash))
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = signedToken

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert,
				TeeAudience:                testAudience,
				TeeAllowedImageIDs:         allowedImageIDsForTest(),
			},
		}
		resp, err := v.DataVerification(context.Background(), teeInfoResponse, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.NoError(t, err)
		require.Equal(t, verifier.OK, resp.Status)
		require.Equal(t, verifier.E2ETestCodeHash, resp.CodeHash)
		require.Equal(t, verifier.E2ETestPlatform, resp.Platform)
	})
	t.Run("empty AllowedImageIDs fails closed (no fail-open skip)", func(t *testing.T) {
		// Defense-in-depth: in the non-E2E path, an empty AllowedImageIDs would
		// make the Policy skip the image_id check. DataVerification must refuse to
		// build the Policy rather than silently disabling enforcement.
		teeInfoResponse, privTEEKey := helpers.TeeInfoResponse(t, challengeHash)
		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert,
				TeeAudience:                testAudience,
				TeeAllowedImageIDs:         map[common.Hash]struct{}{}, // empty
			},
		}
		resp, err := v.DataVerification(context.Background(), teeInfoResponse, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Empty(t, resp)
		require.ErrorContains(t, err, "empty AllowedImageIDs")
	})
	t.Run("empty TeeAudience fails closed (no fail-open skip)", func(t *testing.T) {
		teeInfoResponse, privTEEKey := helpers.TeeInfoResponse(t, challengeHash)
		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert,
				TeeAudience:                "", // empty
				TeeAllowedImageIDs:         allowedImageIDsForTest(),
			},
		}
		resp, err := v.DataVerification(context.Background(), teeInfoResponse, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Empty(t, resp)
		require.ErrorContains(t, err, "empty TeeAudience")
	})
	t.Run("policy rejects mismatched eat_nonce", func(t *testing.T) {
		teeInfoResponse, privTEEKey := helpers.TeeInfoResponse(t, challengeHash)
		// Claims advertise a nonce that does not match the teeInfo hash the
		// verifier computes, so Policy.Apply rejects via eat_nonce.
		baseClaims := validTestClaims("deadbeef")
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = signedToken

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert,
				TeeAudience:                testAudience,
				TeeAllowedImageIDs:         allowedImageIDsForTest(),
			},
		}
		resp, err := v.DataVerification(context.Background(), teeInfoResponse, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Empty(t, resp)
		require.ErrorContains(t, err, "eat_nonce")
	})
	t.Run("expected tee different", func(t *testing.T) {
		teeInfoResponse, privTEEKey := helpers.TeeInfoResponse(t, challengeHash)
		teeInfoHash, err := teeInfoResponse.TeeInfo.Hash()
		require.NoError(t, err)
		baseClaims := validTestClaims(hex.EncodeToString(teeInfoHash))
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = signedToken

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert,
				TeeAudience:                testAudience,
				TeeAllowedImageIDs:         allowedImageIDsForTest(),
			},
		}
		resp, err := v.DataVerification(context.Background(), teeInfoResponse, common.HexToAddress("0x123"))
		require.Empty(t, resp)
		require.ErrorContains(t, err, fmt.Sprintf("expected TEE ID %s, got: %s", common.HexToAddress("0x123"), crypto.PubkeyToAddress(privTEEKey.PublicKey)))
	})
	t.Run("cannot retrieve address from public key", func(t *testing.T) {
		teeInfoResponse, _ := helpers.TeeInfoResponse(t, challengeHash)
		// Corrupt the pubkey before hashing so eat_nonce binds to the same
		// TeeInfo the verifier hashes, and PubKeysToAddresses fails later.
		teeInfoResponse.TeeInfo.PublicKey.X = common.HexToHash("0x1")
		teeInfoHash, err := teeInfoResponse.TeeInfo.Hash()
		require.NoError(t, err)
		baseClaims := validTestClaims(hex.EncodeToString(teeInfoHash))
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = signedToken

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert,
				TeeAudience:                testAudience,
				TeeAllowedImageIDs:         allowedImageIDsForTest(),
			},
		}
		resp, err := v.DataVerification(context.Background(), teeInfoResponse, common.HexToAddress("0x123"))
		require.Empty(t, resp)
		require.ErrorContains(t, err, "cannot retrieve TEE ID from: invalid public key bytes")
	})
}

func TestVerify(t *testing.T) {
	rootCert, leafKey, x5c := generateTestCertificateChain(t)
	verIface, err := verifier.NewVerifier(&config.TeeAvailabilityCheckConfig{
		RPCURL:                     "https://coston-api.flare.network/ext/C/rpc",
		RelayContractAddress:       common.HexToAddress("0x92a6E1127262106611e1e129BB64B6D8654273F7"),
		AllowTeeDebug:              false,
		DisableAttestationCheckE2E: false,
		AllowPrivateNetworks:       true,
		GoogleRootCertificate:      rootCert,
		TeeAudience:                testAudience,
		TeeAllowedImageIDs:         allowedImageIDsForTest(),
	})
	require.NoError(t, err)
	ver, ok := verIface.(*verifier.TeeVerifier)
	require.True(t, ok, "verIface should be *TeeVerifier")
	t.Run("resource not found", func(t *testing.T) {
		teeID := common.HexToAddress("0x123")
		handler := http.NewServeMux()
		handler.HandleFunc("/action/result", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := fdc2.ITeeAvailabilityCheckRequestBody{
			TeeId: teeID,
			Url:   server.URL,
		}
		resp, err := ver.Verify(context.Background(), req)
		require.ErrorContains(t, err, "cannot fetch TEE data for (TEE=0x0000000000000000000000000000000000000123")
		require.ErrorContains(t, err, "action result not found: resource not found (404)")
		require.Empty(t, resp)
	})
	t.Run("signing policy check fails", func(t *testing.T) {
		challengeHash := common.HexToHash("123")
		teeInfoResponse, privTEEKey := helpers.TeeInfoResponse(t, challengeHash)
		teeInfoResponse.TeeInfo.InitialSigningPolicyID = 3000 // invalid signing policy ID
		teeInfoHash, err := teeInfoResponse.TeeInfo.Hash()
		require.NoError(t, err)
		baseClaims := validTestClaims(hex.EncodeToString(teeInfoHash))
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = signedToken
		data, err := json.Marshal(teeInfoResponse)
		require.NoError(t, err)

		privProxyKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		actionResult := teenodetypes.ActionResult{
			Status:    1, // success for direct instructions (tee-node)
			OPType:    op.Reg.Hash(),
			OPCommand: op.TEEAttestation.Hash(),
			Data:      data,
		}
		// helpers.TeeInfoResponse leaves ChainID unset, so the proxy-signature
		// preimage is bound to chainID 0 (matching the recovery side).
		proxySignHash, err := csigning.NewPayload(csigning.ProxyActionResult, 0, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		signature, err := crypto.Sign(accounts.TextHash(proxySignHash[:]), privProxyKey)
		require.NoError(t, err)
		teeSignHash, err := csigning.NewPayload(csigning.TEEActionResult, 0, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		teeSignature, err := crypto.Sign(accounts.TextHash(teeSignHash[:]), privTEEKey)
		require.NoError(t, err)

		handler := http.NewServeMux()
		handler.HandleFunc("/action/result/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := teenodetypes.ActionResponse{
				Result:         actionResult,
				Signature:      teeSignature,
				ProxySignature: signature,
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := fdc2.ITeeAvailabilityCheckRequestBody{
			TeeId:      crypto.PubkeyToAddress(privTEEKey.PublicKey),
			Url:        server.URL,
			TeeProxyId: crypto.PubkeyToAddress(privProxyKey.PublicKey),
			Challenge:  challengeHash,
		}
		resp, err := ver.Verify(context.Background(), req)
		require.ErrorContains(t, err, "failed to validate initial signing policy hash")
		require.Empty(t, resp)
	})
	t.Run("data verification fails", func(t *testing.T) {
		challengeHash := common.HexToHash("123")
		teeInfoResponse, privTEEKey := helpers.TeeInfoResponse(t, challengeHash)
		teeInfoHash, err := teeInfoResponse.TeeInfo.Hash()
		require.NoError(t, err)
		// Claims that satisfy Policy but fail ValidateClaims via SWName.
		baseClaims := validTestClaims(hex.EncodeToString(teeInfoHash))
		baseClaims.SWName = "NOT_CONFIDENTIAL_SPACE"
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = signedToken
		data, err := json.Marshal(teeInfoResponse)
		require.NoError(t, err)

		privProxyKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		actionResult := teenodetypes.ActionResult{
			Status:    1, // success for direct instructions (tee-node)
			OPType:    op.Reg.Hash(),
			OPCommand: op.TEEAttestation.Hash(),
			Data:      data,
		}
		// helpers.TeeInfoResponse leaves ChainID unset, so the proxy-signature
		// preimage is bound to chainID 0 (matching the recovery side).
		proxySignHash, err := csigning.NewPayload(csigning.ProxyActionResult, 0, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		signature, err := crypto.Sign(accounts.TextHash(proxySignHash[:]), privProxyKey)
		require.NoError(t, err)
		teeSignHash, err := csigning.NewPayload(csigning.TEEActionResult, 0, common.BytesToHash(actionResult.Hash())).Hash()
		require.NoError(t, err)
		teeSignature, err := crypto.Sign(accounts.TextHash(teeSignHash[:]), privTEEKey)
		require.NoError(t, err)

		handler := http.NewServeMux()
		handler.HandleFunc("/action/result/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := teenodetypes.ActionResponse{
				Result:         actionResult,
				Signature:      teeSignature,
				ProxySignature: signature,
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := fdc2.ITeeAvailabilityCheckRequestBody{
			TeeId:      crypto.PubkeyToAddress(privTEEKey.PublicKey),
			Url:        server.URL,
			TeeProxyId: crypto.PubkeyToAddress(privProxyKey.PublicKey),
			Challenge:  challengeHash,
		}
		resp, err := ver.Verify(context.Background(), req)
		require.ErrorContains(t, err, "cannot validate claims: SWName check failed")
		require.Empty(t, resp)
	})
}

var _ verifier.RelayCallerInterface = (*MockRelayCaller)(nil)

type MockRelayCaller struct {
	mock.Mock
}

func (m *MockRelayCaller) ToSigningPolicyHash(opts *bind.CallOpts, id *big.Int) ([32]byte, error) {
	args := m.Called(opts, id)
	val, ok := args.Get(0).([32]byte)
	if !ok {
		return [32]byte{}, fmt.Errorf("expected [32]byte, got %T", args.Get(0))
	}
	return val, args.Error(1)
}

func generateTestCertificate(
	t *testing.T,
	notBefore, notAfter time.Time,
	isCA bool,
	parent *x509.Certificate,
	parentKey cr.Signer,
) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serial := big.NewInt(time.Now().UnixNano())

	template := &x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		PublicKeyAlgorithm:    x509.RSA,
		IsCA:                  isCA,
		BasicConstraintsValid: true,
	}

	if isCA {
		template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	} else {
		template.KeyUsage = x509.KeyUsageDigitalSignature
	}

	if parent == nil {
		parent = template
		parentKey = priv
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, &priv.PublicKey, parentKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return cert, priv
}

func generateTestCertificateChain(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, []string) {
	t.Helper()
	rootCert, rootKey := generateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true, nil, nil)
	intermediateCert, intermediateKey := generateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true, rootCert, rootKey)
	leafCert, leafKey := generateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), false, intermediateCert, intermediateKey)
	x5c := []string{
		base64.StdEncoding.EncodeToString(leafCert.Raw),
		base64.StdEncoding.EncodeToString(intermediateCert.Raw),
		base64.StdEncoding.EncodeToString(rootCert.Raw),
	}
	return rootCert, leafKey, x5c
}
