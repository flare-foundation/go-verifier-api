package testhelper

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
