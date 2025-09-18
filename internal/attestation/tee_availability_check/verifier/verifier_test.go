package verifier

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	teetype "github.com/flare-foundation/go-verifier-api/internal/attestation/tee_availability_check/type"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
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
	now := uint64(time.Now().Unix())
	info := testhelper.GetInfoResponse(t)
	challengeBlock := types.NewBlockWithHeader(&types.Header{Time: now - 100})
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
	challengeHash := common.BytesToHash(info.TeeInfo.Challenge)
	challengeBlock, err := v.ethClient.BlockByHash(context.Background(), challengeHash)
	require.NoError(t, err)
	valid, err := v.CheckInfoChallengeIsValid(context.Background(), challengeHash)
	require.NoError(t, err)
	require.Equal(t, valid, teetype.TeePollerSampleValid)
}

type MockRelayCaller struct {
	mock.Mock
}

func (m *MockRelayCaller) ToSigningPolicyHash(opts *bind.CallOpts, id *big.Int) ([32]byte, error) {
	args := m.Called(opts, id)
	return args.Get(0).([32]byte), args.Error(1)
}

func TestTeeVerifier_getSigningPolicyHashFromChain(t *testing.T) {
	mockRelay := &MockRelayCaller{}
	v := &TeeVerifier{
		RelayCaller: mockRelay,
	}

	t.Run("success", func(t *testing.T) {
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
		mockRelay.On("ToSigningPolicyHash", mock.Anything, big.NewInt(99)).Return([32]byte{}, errors.New("rpc error"))

		_, _, err := v.getSigningPolicyHashFromChain(context.Background(), 99)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown error")
		mockRelay.AssertExpectations(t)
	})
}
