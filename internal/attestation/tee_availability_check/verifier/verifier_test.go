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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier"
	verifiertypes "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/verifier/types"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/flare-foundation/go-verifier-api/internal/tests/helpers"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCheckInfoChallenge(t *testing.T) {
	// #nosec G115: only used in test, integer overflow not a concern
	now := uint64(time.Now().Unix())
	challengeHash := common.HexToHash("0x123")

	t.Run("valid", func(t *testing.T) {
		challengeBlock := types.NewBlockWithHeader(&types.Header{Time: now - 10})
		latestBlock := types.NewBlockWithHeader(&types.Header{Time: now})
		mockClient := &helpers.MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return challengeBlock, nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return latestBlock, nil
			},
		}
		v := &verifier.TeeVerifier{EthClient: mockClient}

		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.NoError(t, err)
		require.Equal(t, verifiertypes.TeeSampleValid, state)
	})
	t.Run("challenge block fetch fails", func(t *testing.T) {
		mockClient := &helpers.MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return nil, errors.New("rpc error")
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return types.NewBlockWithHeader(&types.Header{Time: now}), nil
			},
		}
		v := &verifier.TeeVerifier{EthClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "fetch challenge block: unknown error")
		require.NotEqual(t, verifiertypes.TeeSampleValid, state)
	})
	t.Run("latest block fetch fails with ErrUnknown", func(t *testing.T) {
		mockClient := &helpers.MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return types.NewBlockWithHeader(&types.Header{Time: now - 10}), nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return nil, verifiertypes.ErrUnknown
			},
		}
		v := &verifier.TeeVerifier{EthClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "fetch latest block: unknown error")
		require.Equal(t, verifiertypes.TeeSampleIndeterminate, state)
	})
	t.Run("latest block fetch fails with other error", func(t *testing.T) {
		mockClient := &helpers.MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return types.NewBlockWithHeader(&types.Header{Time: now - 10}), nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return nil, errors.New("rpc failure")
			},
		}
		v := &verifier.TeeVerifier{EthClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "fetch latest block: unknown error")
		require.NotEqual(t, verifiertypes.TeeSampleValid, state)
	})
	t.Run("challenge too old", func(t *testing.T) {
		challengeBlock := types.NewBlockWithHeader(&types.Header{Time: now - (verifier.BlockFreshnessInSeconds + 10)})
		latestBlock := types.NewBlockWithHeader(&types.Header{Time: now})
		mockClient := &helpers.MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return challengeBlock, nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return latestBlock, nil
			},
		}
		v := &verifier.TeeVerifier{EthClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "challenge too old")
		require.Equal(t, verifiertypes.TeeSampleInvalid, state)
	})
}

func TestGetSigningPolicyHashFromChain(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{
			RelayCaller: mockRelay,
		}
		expectedHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
		var hashBytes [32]byte
		copy(hashBytes[:], expectedHash.Bytes())
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(42)).Return(hashBytes, nil)
		hash, _, err := v.GetSigningPolicyHashFromChain(context.Background(), 42)
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
		_, _, err := v.GetSigningPolicyHashFromChain(context.Background(), 99)
		require.ErrorContains(t, err, "ToSigningPolicyHash: unknown error")
		mockRelay.AssertExpectations(t)
	})
}

func TestGetSigningPolicyHashFromChainWithRetry(t *testing.T) {
	expectedHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	var hashBytes [32]byte
	copy(hashBytes[:], expectedHash.Bytes())
	maxAttempts := 2
	delay := 150 * time.Millisecond

	t.Run("success on first attempt", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &verifier.TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(42)).Return(hashBytes, nil)
		hash, state, err := v.GetSigningPolicyHashFromChainWithRetry(context.Background(), 42, maxAttempts, delay)
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
		hash, state, err := v.GetSigningPolicyHashFromChainWithRetry(context.Background(), 43, maxAttempts, delay)
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
		hash, state, err := v.GetSigningPolicyHashFromChainWithRetry(context.Background(), 99, maxAttempts, delay)
		require.ErrorContains(t, err, "getSigningPolicyHashFromChainWithRetry failed after 2 attempts: ToSigningPolicyHash: unknown error")
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

func TestIsTEEInfoDown(t *testing.T) {
	teeID := common.HexToAddress("0x1")
	now := time.Now()
	t.Run("insufficient samples", func(t *testing.T) {
		v := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				teeID: {{Timestamp: now, State: verifiertypes.TeeSampleValid}},
			},
		}
		down, err := v.IsTEEInfoDown(teeID)
		require.ErrorContains(t, err, "insufficient samples to determine TEE")
		require.False(t, down)
	})
	t.Run("at least one valid sample", func(t *testing.T) {
		v := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				teeID: {
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
					{Timestamp: now, State: verifiertypes.TeeSampleValid},
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
					{Timestamp: now, State: verifiertypes.TeeSampleIndeterminate},
				},
			},
		}
		down, err := v.IsTEEInfoDown(teeID)
		require.NoError(t, err)
		require.False(t, down)
	})
	t.Run("all samples invalid", func(t *testing.T) {
		v := &verifier.TeeVerifier{
			TeeSamples: map[common.Address][]verifiertypes.TeeSampleValue{
				teeID: {
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
					{Timestamp: now, State: verifiertypes.TeeSampleInvalid},
				},
			},
		}

		down, err := v.IsTEEInfoDown(teeID)
		require.NoError(t, err)
		require.True(t, down)
	})
}

func TestFetchTEEChallengeResult(t *testing.T) {
	ctx := context.Background()
	baseURL := "http://base"
	challengeID := common.HexToHash("0x123")
	t.Run("success", func(t *testing.T) {
		validJSON := `{"teeInfo":{"InitialSigningPolicyID":1}}`
		data := hexutil.Bytes([]byte(validJSON))

		privKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		address := crypto.PubkeyToAddress(privKey.PublicKey)
		hash := crypto.Keccak256(data)
		ethHash := accounts.TextHash(hash)
		signature, err := crypto.Sign(ethHash, privKey)
		require.NoError(t, err)

		mockFetchFn := func(ctx context.Context, url string, timeout time.Duration) (teenodetypes.ActionResponse, error) {
			return teenodetypes.ActionResponse{
				Result: teenodetypes.ActionResult{
					Data: data,
				},
				ProxySignature: signature,
			}, nil
		}
		teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.NotEqual(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, address, signer)
		require.NoError(t, err)
	})
	t.Run("fetch error", func(t *testing.T) {
		mockFetchFn := func(ctx context.Context, url string, timeout time.Duration) (teenodetypes.ActionResponse, error) {
			return teenodetypes.ActionResponse{}, errors.New("bad request")
		}
		teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "bad request")
	})
	t.Run("empty data", func(t *testing.T) {
		mockFetchFn := func(ctx context.Context, url string, timeout time.Duration) (teenodetypes.ActionResponse, error) {
			response := teenodetypes.ActionResponse{
				Result: teenodetypes.ActionResult{
					Data: hexutil.Bytes{},
				},
			}
			return response, nil
		}
		teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "TEE challenge result data is empty")
	})
	t.Run("invalid JSON data", func(t *testing.T) {
		mockFetchFn := func(ctx context.Context, url string, timeout time.Duration) (teenodetypes.ActionResponse, error) {
			response := teenodetypes.ActionResponse{
				Result: teenodetypes.ActionResult{
					Data: hexutil.Bytes([]byte("not-json")),
				},
			}
			return response, nil
		}
		teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, `TEE challenge result data is not valid JSON`)
	})
	t.Run("unmarshal error", func(t *testing.T) {
		mockFetchFn := func(ctx context.Context, url string, timeout time.Duration) (teenodetypes.ActionResponse, error) {
			badJSON := `{"teeInfo":"this-should-be-an-object-not-a-string"}`
			return teenodetypes.ActionResponse{
				Result: teenodetypes.ActionResult{
					Data: hexutil.Bytes([]byte(badJSON)),
				},
			}, nil
		}
		teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "unmarshal TEE result")
	})
	t.Run("recover signer error", func(t *testing.T) {
		mockFetchFn := func(ctx context.Context, url string, timeout time.Duration) (teenodetypes.ActionResponse, error) {
			validJSON := `{"teeInfo":{"InitialSigningPolicyID":1}}`
			return teenodetypes.ActionResponse{
				Result: teenodetypes.ActionResult{
					Data: hexutil.Bytes([]byte(validJSON)),
				},
				ProxySignature: []byte("invalid-signature"),
			}, nil
		}
		teeInfo, signer, err := verifier.FetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "recover signer")
	})
}

func TestDataVerification(t *testing.T) {
	rootCert, leafKey, x5c := generateTestCertificateChain(t)
	challengeHash := common.HexToHash("123")
	t.Run("DisableAttestationCheckE2E", func(t *testing.T) {
		v := &verifier.TeeVerifier{Cfg: &config.TeeAvailabilityCheckConfig{DisableAttestationCheckE2E: true}}
		res, err := v.DataVerification(teenodetypes.TeeInfoResponse{}, common.Address{})
		require.NoError(t, err)
		require.Equal(t, verifier.E2ETestCodeHash, res.CodeHash)
		require.Equal(t, verifier.E2ETestPlatform, res.Platform)
	})
	t.Run("success", func(t *testing.T) {
		teeInfoResponse, privTEEKey := helpers.GetTeeInfoResponse(t, challengeHash)
		teeInfoHash, err := teeInfoResponse.TeeInfo.Hash()
		require.NoError(t, err)
		baseClaims := &googlecloud.GoogleTeeClaims{
			HWModel:     "TEST_PLATFORM",
			SWName:      "CONFIDENTIAL_SPACE",
			EATNonce:    []string{hex.EncodeToString(teeInfoHash)},
			DebugStatus: "disabled-since-boot",
			SubMods: googlecloud.SubMods{
				ConfidentialSpace: googlecloud.ConfidentialSpaceInfo{
					SupportAttributes: []string{"STABLE"},
				},
				Container: googlecloud.Container{
					ImageDigest: "sha256:194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2",
				},
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = hexutil.Bytes([]byte(signedToken))

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert},
		}
		resp, err := v.DataVerification(teeInfoResponse, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.NoError(t, err)
		require.Equal(t, verifier.OK, resp.Status)
		require.Equal(t, verifier.E2ETestCodeHash, resp.CodeHash)
		require.Equal(t, verifier.E2ETestPlatform, resp.Platform)
	})
	t.Run("invalid claims", func(t *testing.T) {
		teeInfoResponse, privTEEKey := helpers.GetTeeInfoResponse(t, challengeHash)
		baseClaims := &googlecloud.GoogleTeeClaims{
			HWModel: "GCP_INTEL_TDX",
			SWName:  "CONFIDENTIAL_SPACE",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = hexutil.Bytes([]byte(signedToken))

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert},
		}
		resp, err := v.DataVerification(teeInfoResponse, crypto.PubkeyToAddress(privTEEKey.PublicKey))
		require.Empty(t, resp)
		require.ErrorContains(t, err, "cannot validate claims: expected exactly one EATNonce, got 0")
	})
	t.Run("expected tee different", func(t *testing.T) {
		teeInfoResponse, privTEEKey := helpers.GetTeeInfoResponse(t, challengeHash)
		baseClaims := &googlecloud.GoogleTeeClaims{
			HWModel: "GCP_INTEL_TDX",
			SWName:  "CONFIDENTIAL_SPACE",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = hexutil.Bytes([]byte(signedToken))

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert},
		}
		resp, err := v.DataVerification(teeInfoResponse, common.HexToAddress("0x123"))
		require.Empty(t, resp)
		require.ErrorContains(t, err, fmt.Sprintf("expected TEE ID %s, got: %s", common.HexToAddress("0x123"), crypto.PubkeyToAddress(privTEEKey.PublicKey)))
	})
	t.Run("cannot retrieve address from public key", func(t *testing.T) {
		teeInfoResponse, _ := helpers.GetTeeInfoResponse(t, challengeHash)
		teeInfoResponse.TeeInfo.PublicKey.X = common.HexToHash("0x1")
		baseClaims := &googlecloud.GoogleTeeClaims{
			HWModel: "GCP_INTEL_TDX",
			SWName:  "CONFIDENTIAL_SPACE",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = hexutil.Bytes([]byte(signedToken))

		v := &verifier.TeeVerifier{
			Cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      rootCert},
		}
		resp, err := v.DataVerification(teeInfoResponse, common.HexToAddress("0x123"))
		require.Empty(t, resp)
		require.ErrorContains(t, err, "cannot retrieve TEE ID from: invalid public key bytes")
	})
}

func TestVerify(t *testing.T) {
	rootCert, leafKey, x5c := generateTestCertificateChain(t)
	verIface, err := verifier.NewVerifier(&config.TeeAvailabilityCheckConfig{
		RPCURL:                            "https://coston-api.flare.network/ext/C/rpc",
		RelayContractAddress:              "0x92a6E1127262106611e1e129BB64B6D8654273F7",
		TeeMachineRegistryContractAddress: "0x053568617FFccEe2F75073975CC0e1549Ff9db71",
		AllowTeeDebug:                     false,
		DisableAttestationCheckE2E:        false,
		GoogleRootCertificate:             rootCert,
	})
	require.NoError(t, err)
	ver, ok := verIface.(*verifier.TeeVerifier)
	require.True(t, ok, "verIface should be *TeeVerifier")
	ver.TeeSamples = make(map[common.Address][]verifiertypes.TeeSampleValue)
	t.Run("FetchTEEChallengeResult error", func(t *testing.T) {
		handler := http.NewServeMux()
		handler.HandleFunc("/action/result/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := connector.ITeeAvailabilityCheckRequestBody{
			Url: server.URL,
		}
		resp, err := ver.Verify(context.Background(), req)
		require.ErrorContains(t, err, "cannot fetch TEE data for TeeID")
		require.Empty(t, resp)
	})
	t.Run("insufficient samples", func(t *testing.T) {
		handler := http.NewServeMux()
		handler.HandleFunc("/action/result", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := connector.ITeeAvailabilityCheckRequestBody{
			Url: server.URL,
		}
		resp, err := ver.Verify(context.Background(), req)
		require.ErrorContains(t, err, "insufficient samples to determine TEE")
		require.Empty(t, resp)
	})
	t.Run("indeterminate status", func(t *testing.T) {
		teeID := common.HexToAddress("0x123")
		handler := http.NewServeMux()
		handler.HandleFunc("/action/result", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := connector.ITeeAvailabilityCheckRequestBody{
			TeeId: teeID,
			Url:   server.URL,
		}
		ver.TeeSamples[teeID] = []verifiertypes.TeeSampleValue{{State: verifiertypes.TeeSampleValid}, {State: verifiertypes.TeeSampleInvalid}, {State: verifiertypes.TeeSampleInvalid}, {State: verifiertypes.TeeSampleInvalid}, {State: verifiertypes.TeeSampleInvalid}}
		resp, err := ver.Verify(context.Background(), req)
		require.ErrorContains(t, err, "indeterminate verifier status")
		require.Empty(t, resp)
		// reset samples
		ver.TeeSamples[teeID] = []verifiertypes.TeeSampleValue{}
	})
	t.Run("tee is down", func(t *testing.T) {
		teeID := common.HexToAddress("0x123")
		handler := http.NewServeMux()
		handler.HandleFunc("/action/result", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := connector.ITeeAvailabilityCheckRequestBody{
			TeeId: teeID,
			Url:   server.URL,
		}
		ver.TeeSamples[teeID] = []verifiertypes.TeeSampleValue{{State: verifiertypes.TeeSampleInvalid}, {State: verifiertypes.TeeSampleInvalid}, {State: verifiertypes.TeeSampleInvalid}, {State: verifiertypes.TeeSampleInvalid}, {State: verifiertypes.TeeSampleInvalid}}
		resp, err := ver.Verify(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, uint8(verifier.DOWN), resp.Status)
		// reset samples
		ver.TeeSamples[teeID] = []verifiertypes.TeeSampleValue{}
	})
	t.Run("signing policy check fails", func(t *testing.T) {
		challengeHash := common.HexToHash("123")
		teeInfoResponse, privTEEKey := helpers.GetTeeInfoResponse(t, challengeHash)
		teeInfoResponse.TeeInfo.InitialSigningPolicyID = 3000 // invalid signing policy ID
		teeInfoHash, err := teeInfoResponse.TeeInfo.Hash()
		require.NoError(t, err)
		baseClaims := &googlecloud.GoogleTeeClaims{
			HWModel:     "TEST_PLATFORM",
			SWName:      "CONFIDENTIAL_SPACE",
			EATNonce:    []string{hex.EncodeToString(teeInfoHash)},
			DebugStatus: "disabled-since-boot",
			SubMods: googlecloud.SubMods{
				ConfidentialSpace: googlecloud.ConfidentialSpaceInfo{
					SupportAttributes: []string{"STABLE"},
				},
				Container: googlecloud.Container{
					ImageDigest: "sha256:194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2",
				},
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = hexutil.Bytes([]byte(signedToken))
		data, err := json.Marshal(teeInfoResponse)
		require.NoError(t, err)

		privProxyKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		hash := crypto.Keccak256(data)
		ethHash := accounts.TextHash(hash)
		signature, err := crypto.Sign(ethHash, privProxyKey)
		require.NoError(t, err)

		handler := http.NewServeMux()
		handler.HandleFunc("/action/result/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := teenodetypes.ActionResponse{
				Result: teenodetypes.ActionResult{
					Data: data,
				},
				ProxySignature: signature,
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := connector.ITeeAvailabilityCheckRequestBody{
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
		teeInfoResponse, privTEEKey := helpers.GetTeeInfoResponse(t, challengeHash)
		baseClaims := &googlecloud.GoogleTeeClaims{
			HWModel: "GCP_INTEL_TDX",
			SWName:  "CONFIDENTIAL_SPACE",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, baseClaims)
		token.Header["x5c"] = x5c
		signedToken, err := token.SignedString(leafKey)
		require.NoError(t, err)
		teeInfoResponse.Attestation = hexutil.Bytes([]byte(signedToken))
		data, err := json.Marshal(teeInfoResponse)
		require.NoError(t, err)

		privProxyKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		hash := crypto.Keccak256(data)
		ethHash := accounts.TextHash(hash)
		signature, err := crypto.Sign(ethHash, privProxyKey)
		require.NoError(t, err)

		handler := http.NewServeMux()
		handler.HandleFunc("/action/result/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := teenodetypes.ActionResponse{
				Result: teenodetypes.ActionResult{
					Data: data,
				},
				ProxySignature: signature,
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		})
		server := httptest.NewServer(handler)
		defer server.Close()

		req := connector.ITeeAvailabilityCheckRequestBody{
			TeeId:      crypto.PubkeyToAddress(privTEEKey.PublicKey),
			Url:        server.URL,
			TeeProxyId: crypto.PubkeyToAddress(privProxyKey.PublicKey),
			Challenge:  challengeHash,
		}
		resp, err := ver.Verify(context.Background(), req)
		require.ErrorContains(t, err, "cannot validate claims: expected exactly one EATNonce, got 0")
		require.Empty(t, resp)
	})
}

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
