package verifier

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/coreutil"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	teenodetypes "github.com/flare-foundation/tee-node/pkg/types"
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
	info := testhelper.GetInfoResponse(t)
	challengeHash := common.BytesToHash(info.TeeInfo.Challenge)

	t.Run("valid challenge (fresh)", func(t *testing.T) {
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

	t.Run("success on first attempt", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(42)).Return(hashBytes, nil)
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(context.Background(), 42)
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
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(context.Background(), 43)
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
		hash, state, err := v.getSigningPolicyHashFromChainWithRetry(context.Background(), 99)
		require.ErrorContains(t, err, "getSigningPolicyHashFromChainWithRetry failed after 2 retries: ToSigningPolicyHash: unknown error")
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

	t.Run("success", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		teeInfo := teenodetypes.TeeInfo{
			InitialSigningPolicyID:   1,
			InitialSigningPolicyHash: expectedInitialHash,
			LastSigningPolicyID:      2,
			LastSigningPolicyHash:    expectedLastHash,
		}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), teeInfo)
		require.NoError(t, err)
		require.Equal(t, teetype.TeePollerSampleValid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("initial hash mismatch", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		teeInfo := teenodetypes.TeeInfo{
			InitialSigningPolicyID:   1,
			InitialSigningPolicyHash: common.HexToHash("0xdeadbeef"),
		}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), teeInfo)
		require.ErrorContains(t, err, "failed to validate initial signing policy hash")
		require.Equal(t, teetype.TeePollerSampleInvalid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("last hash mismatch", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		teeInfo := teenodetypes.TeeInfo{
			InitialSigningPolicyID:   1,
			InitialSigningPolicyHash: expectedInitialHash,
			LastSigningPolicyID:      2,
			LastSigningPolicyHash:    common.HexToHash("0xdeadbeef"),
		}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return(lastBytes, nil)
		state, err := v.CheckSigningPolicies(context.Background(), teeInfo)
		require.ErrorContains(t, err, "failed to validate last signing policy hash")
		require.Equal(t, teetype.TeePollerSampleInvalid, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("fail to retrieve initial hash", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		teeInfo := teenodetypes.TeeInfo{
			InitialSigningPolicyID: 1,
		}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return([32]byte{}, errors.New("rpc error"))
		state, err := v.CheckSigningPolicies(context.Background(), teeInfo)
		require.ErrorContains(t, err, "failed to retrieve initial signing policy hash")
		require.Equal(t, teetype.TeePollerSampleIndeterminate, state)
		mockRelay.AssertExpectations(t)
	})
	t.Run("fail to retrieve last hash", func(t *testing.T) {
		mockRelay := &MockRelayCaller{}
		v := &TeeVerifier{RelayCaller: mockRelay}
		teeInfo := teenodetypes.TeeInfo{
			InitialSigningPolicyID:   1,
			InitialSigningPolicyHash: expectedInitialHash,
			LastSigningPolicyID:      2,
		}
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(1)).Return(initialBytes, nil)
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(2)).Return([32]byte{}, errors.New("rpc error"))
		state, err := v.CheckSigningPolicies(context.Background(), teeInfo)
		require.ErrorContains(t, err, "failed to retrieve last signing policy hash")
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
			SamplesToConsider: 3,
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
				},
			},
			SamplesToConsider: 3,
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
				},
			},
			SamplesToConsider: 3,
		}

		down, err := v.isTEEInfoDown(teeID)
		require.NoError(t, err)
		require.True(t, down)
	})
}

func TestRecoverSigner(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	t.Run("valid signature", func(t *testing.T) {
		expectedAddr := crypto.PubkeyToAddress(privKey.PublicKey)

		data := []byte("hello world")
		hash := crypto.Keccak256(data)
		signature, err := crypto.Sign(accounts.TextHash(hash), privKey)
		require.NoError(t, err)

		addr, err := recoverSigner(data, signature)
		require.NoError(t, err)
		require.Equal(t, expectedAddr, addr)
	})
	t.Run("invalid signature", func(t *testing.T) {
		data := []byte("hello")
		invalidSig := []byte("notavalidsignature")

		addr, err := recoverSigner(data, invalidSig)
		require.ErrorContains(t, err, "failed to recover pubkey: invalid signature length")
		require.Equal(t, common.Address{}, addr)
	})
	t.Run("empty data", func(t *testing.T) {
		signature, err := crypto.Sign(accounts.TextHash(crypto.Keccak256([]byte{})), privKey)
		require.NoError(t, err)

		addr, err := recoverSigner([]byte{}, signature)
		require.NoError(t, err)
		require.Equal(t, crypto.PubkeyToAddress(privKey.PublicKey), addr)
	})
	t.Run("truncated signature", func(t *testing.T) {
		data := []byte("hello world")
		hash := crypto.Keccak256(data)
		signature, err := crypto.Sign(accounts.TextHash(hash), privKey)
		require.NoError(t, err)

		// Remove last byte to make it invalid
		truncatedSig := signature[:len(signature)-1]

		addr, err := recoverSigner(data, truncatedSig)
		require.ErrorContains(t, err, "failed to recover pubkey: invalid signature length")
		require.Equal(t, common.Address{}, addr)
	})
}
