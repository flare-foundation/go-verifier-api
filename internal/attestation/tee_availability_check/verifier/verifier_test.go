package verifier

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockEthClient struct {
	BlockByHashFn   func(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockByNumberFn func(ctx context.Context, number *big.Int) (*types.Block, error)
}

func (m *MockEthClient) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return m.BlockByHashFn(ctx, hash)
}

func (m *MockEthClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return m.BlockByNumberFn(ctx, number)
}

func TestCheckInfoChallengeIsValid(t *testing.T) {
	// #nosec G115: only used in test, integer overflow not a concern
	now := uint64(time.Now().Unix())
	challengeHash := common.HexToHash("0x123")

	t.Run("valid", func(t *testing.T) {
		challengeBlock := types.NewBlockWithHeader(&types.Header{Time: now - 10})
		latestBlock := types.NewBlockWithHeader(&types.Header{Time: now})
		mockClient := &MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return challengeBlock, nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return latestBlock, nil
			},
		}
		v := &TeeVerifier{ethClient: mockClient}

		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.NoError(t, err)
		require.Equal(t, teetype.TeePollerSampleValid, state)
	})
	t.Run("challenge block fetch fails", func(t *testing.T) {
		mockClient := &MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return nil, errors.New("rpc error")
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return types.NewBlockWithHeader(&types.Header{Time: now}), nil
			},
		}
		v := &TeeVerifier{ethClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "fetch challenge block: unknown error")
		require.NotEqual(t, teetype.TeePollerSampleValid, state)
	})
	t.Run("latest block fetch fails with ErrInvalidInput", func(t *testing.T) {
		mockClient := &MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return types.NewBlockWithHeader(&types.Header{Time: now - 10}), nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return nil, coreutil.ErrInvalidInput
			},
		}
		v := &TeeVerifier{ethClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "fetch latest block: invalid input")
		require.Equal(t, teetype.TeePollerSampleIndeterminate, state)
	})
	t.Run("latest block fetch fails with other error", func(t *testing.T) {
		mockClient := &MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return types.NewBlockWithHeader(&types.Header{Time: now - 10}), nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return nil, errors.New("rpc failure")
			},
		}
		v := &TeeVerifier{ethClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "fetch latest block: unknown error")
		require.NotEqual(t, teetype.TeePollerSampleValid, state)
	})
	t.Run("challenge too old", func(t *testing.T) {
		challengeBlock := types.NewBlockWithHeader(&types.Header{Time: now - (blockFreshnessInSeconds + 10)})
		latestBlock := types.NewBlockWithHeader(&types.Header{Time: now})
		mockClient := &MockEthClient{
			BlockByHashFn: func(ctx context.Context, hash common.Hash) (*types.Block, error) {
				return challengeBlock, nil
			},
			BlockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return latestBlock, nil
			},
		}
		v := &TeeVerifier{ethClient: mockClient}
		state, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
		require.ErrorContains(t, err, "challenge too old")
		require.Equal(t, teetype.TeePollerSampleInvalid, state)
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

func TestTeeVerifier_getSigningPolicyHashFromChain(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{
			RelayCaller: mockRelay,
		}
		expectedHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
		var hashBytes [32]byte
		copy(hashBytes[:], expectedHash.Bytes())
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(42)).Return(hashBytes, nil)
		hash, _, err := v.getSigningPolicyHashFromChain(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, expectedHash, hash)
		mockRelay.AssertExpectations(t)
	})
	t.Run("failure", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{
			RelayCaller: mockRelay,
		}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(99)).Return([32]byte{}, errors.New("rpc error"))
		_, _, err := v.getSigningPolicyHashFromChain(context.Background(), 99)
		require.ErrorContains(t, err, "ToSigningPolicyHash: unknown error")
		mockRelay.AssertExpectations(t)
	})
}

func TestTeeVerifier_getSigningPolicyHashFromChainWithRetry(t *testing.T) {
	expectedHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	var hashBytes [32]byte
	copy(hashBytes[:], expectedHash.Bytes())
	maxAttempts := 2
	delay := 150 * time.Millisecond

	t.Run("success on first attempt", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(42)).Return(hashBytes, nil)
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(context.Background(), 42, maxAttempts, delay)
		require.NoError(t, err)
		require.Equal(t, teetype.TeePollerSampleValid, state)
		require.Equal(t, expectedHash, hash)
		mockRelay.AssertExpectations(t)
	})
	t.Run("succeeds after retry", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
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
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(context.Background(), 43, maxAttempts, delay)
		require.NoError(t, err)
		require.Equal(t, teetype.TeePollerSampleValid, state)
		require.Equal(t, expectedHash, hash)
		require.GreaterOrEqual(t, callCount, 2, "should retry at least once")
		mockRelay.AssertExpectations(t)
	})
	t.Run("fails after all retries", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(99)).Return([32]byte{}, errors.New("rpc error"))
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(context.Background(), 99, maxAttempts, delay)
		require.ErrorContains(t, err, "getSigningPolicyHashFromChainWithRetry failed after 2 attempts: ToSigningPolicyHash: unknown error")
		require.Equal(t, teetype.TeePollerSampleIndeterminate, state)
		require.Equal(t, common.Hash{}, hash)
		mockRelay.AssertExpectations(t)
	})
}

func TestTeeVerifier_CheckSigningPolicies(t *testing.T) {
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
		v := &TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), baseTEEInfo)
		require.NoError(t, err)
		require.Equal(t, teetype.TeePollerSampleValid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("initial hash mismatch", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		modTEEInfo := baseTEEInfo
		modTEEInfo.InitialSigningPolicyHash = common.HexToHash("0xdeadbeef")
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), modTEEInfo)
		require.ErrorContains(t, err, "failed to validate initial signing policy hash")
		require.Equal(t, teetype.TeePollerSampleInvalid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("last hash mismatch", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		modTEEInfo := baseTEEInfo
		modTEEInfo.LastSigningPolicyHash = common.HexToHash("0xdeadbeef")
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), modTEEInfo)
		require.ErrorContains(t, err, "failed to validate last signing policy hash")
		require.Equal(t, teetype.TeePollerSampleInvalid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("fail to retrieve initial hash", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return([32]byte{}, errors.New("rpc error"))
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), baseTEEInfo)
		require.ErrorContains(t, err, "cannot retrieve initial signing policy hash for ID 1")
		require.Equal(t, teetype.TeePollerSampleIndeterminate, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("fail to retrieve last hash", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return([32]byte{}, errors.New("rpc error"))
		state, err := v.CheckSigningPolicies(context.Background(), baseTEEInfo)
		require.ErrorContains(t, err, "cannot retrieve last signing policy hash for ID 2")
		require.Equal(t, teetype.TeePollerSampleIndeterminate, state)
		mockRelay.AssertExpectations(t)
	})
}

func TestTeeVerifier_isTEEInfoDown(t *testing.T) {
	teeID := common.HexToAddress("0x1")
	now := time.Now()
	t.Run("insufficient samples", func(t *testing.T) {
		v := &TeeVerifier{
			TeeSamples: map[common.Address][]teetype.TeePollerSample{
				teeID: {{Timestamp: now, State: teetype.TeePollerSampleValid}},
			},
		}
		down, err := v.isTEEInfoDown(teeID)
		require.ErrorContains(t, err, "insufficient samples to determine TEE")
		require.False(t, down)
	})
	t.Run("at least one valid sample", func(t *testing.T) {
		v := &TeeVerifier{
			TeeSamples: map[common.Address][]teetype.TeePollerSample{
				teeID: {
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
					{Timestamp: now, State: teetype.TeePollerSampleValid},
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
					{Timestamp: now, State: teetype.TeePollerSampleIndeterminate},
				},
			},
		}
		down, err := v.isTEEInfoDown(teeID)
		require.NoError(t, err)
		require.False(t, down)
	})
	t.Run("all samples invalid", func(t *testing.T) {
		v := &TeeVerifier{
			TeeSamples: map[common.Address][]teetype.TeePollerSample{
				teeID: {
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
					{Timestamp: now, State: teetype.TeePollerSampleInvalid},
				},
			},
		}

		down, err := v.isTEEInfoDown(teeID)
		require.NoError(t, err)
		require.True(t, down)
	})
}

func TestTeeVerifier_fetchTEEChallengeResult(t *testing.T) {
	ctx := context.Background()
	baseURL := "http://example.com"
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
		teeInfo, signer, err := fetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.NotEqual(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, address, signer)
		require.NoError(t, err)
	})
	t.Run("fetch error", func(t *testing.T) {
		mockFetchFn := func(ctx context.Context, url string, timeout time.Duration) (teenodetypes.ActionResponse, error) {
			return teenodetypes.ActionResponse{}, errors.New("bad request")
		}
		teeInfo, signer, err := fetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
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
		teeInfo, signer, err := fetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
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
		teeInfo, signer, err := fetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
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
		teeInfo, signer, err := fetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
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
		teeInfo, signer, err := fetchTEEChallengeResult(ctx, baseURL, challengeID, mockFetchFn)
		require.Equal(t, teenodetypes.TeeInfoResponse{}, teeInfo)
		require.Equal(t, common.Address{}, signer)
		require.ErrorContains(t, err, "recover signer")
	})
}

func TestDataVerification(t *testing.T) {
	t.Run("DisableAttestationCheckE2E", func(t *testing.T) {
		v := &TeeVerifier{cfg: &config.TeeAvailabilityCheckConfig{DisableAttestationCheckE2E: true}}
		res, err := v.DataVerification(teenodetypes.TeeInfoResponse{}, common.Address{})
		require.NoError(t, err)
		require.Equal(t, common.HexToHash("194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2"), res.CodeHash)
		require.Equal(t, common.HexToHash("544553545f504c4154464f524d00000000000000000000000000000000000000"), res.Platform)
	})
	t.Run("valid input fails on chain", func(t *testing.T) {
		cert := testhelper.GenerateTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
		v := &TeeVerifier{
			cfg: &config.TeeAvailabilityCheckConfig{
				DisableAttestationCheckE2E: false,
				GoogleRootCertificate:      cert},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"foo": "bar"})
		token.Header["x5c"] = []string{"invalid-cert", "invalid-cert", "invalid-cert"}
		privKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		signedToken, err := token.SignedString(privKey)
		require.NoError(t, err)

		teeInfo := teenodetypes.TeeInfoResponse{
			TeeInfo:     teenodetypes.TeeInfo{},
			Attestation: hexutil.Bytes([]byte(signedToken)),
		}
		_, err = v.DataVerification(teeInfo, common.Address{})
		require.ErrorContains(t, err, "cannot validate certificate signature: parsing and verifying: token is unverifiable: error while executing keyfunc: extracting certificates from x5c headers: cannot parse certificate at index 0: cannot decode certificate illegal base64 data at input byte 7")
	})
}
