package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

const testAddress = "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL"

func TestGetAccountInfo(t *testing.T) {
	t.Run("Invalid rpc url", func(t *testing.T) {
		client := NewClient("https://invalid-rpccom")
		ctx := context.Background()
		_, err := client.GetAccountInfo(ctx, testAddress)
		require.Error(t, err)
	})

	t.Run("Invalid address", func(t *testing.T) {
		client := NewClient("https://s.altnet.rippletest.net:51234")
		ctx := context.Background()
		_, err := client.GetAccountInfo(ctx, "0x")
		require.Error(t, err)
	})

	t.Run("Valid address", func(t *testing.T) {
		client := NewClient("https://s.altnet.rippletest.net:51234")
		ctx := context.Background()
		resp, err := client.GetAccountInfo(ctx, testAddress)
		require.NoError(t, err)
		require.Equal(t, testAddress, resp.Result.AccountData.Account)
		require.NotZero(t, resp.Result.AccountData.Sequence)
		require.GreaterOrEqual(t, len(resp.Result.AccountData.SignerLists), 1)
	})
}
