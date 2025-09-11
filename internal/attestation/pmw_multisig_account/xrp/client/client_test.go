package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAccountInfo(t *testing.T) {
	t.Run("Invalid rpc url", func(t *testing.T) {
		client := NewClient("https://invalid-rpccom")
		ctx := context.Background()
		resp, err := client.GetAccountInfo(ctx, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL")
		require.Error(t, err)
		_ = resp
	})

	t.Run("Invalid address", func(t *testing.T) {
		client := NewClient("https://s.altnet.rippletest.net:51234")
		ctx := context.Background()
		resp, err := client.GetAccountInfo(ctx, "0x")
		require.Error(t, err)
		_ = resp
	})

	t.Run("Valid address", func(t *testing.T) {
		client := NewClient("https://s.altnet.rippletest.net:51234")
		ctx := context.Background()
		resp, err := client.GetAccountInfo(ctx, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL")
		require.NoError(t, err)
		require.Equal(t, "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL", resp.Result.AccountData.Account)
		require.NotZero(t, resp.Result.AccountData.Sequence)
		require.GreaterOrEqual(t, len(resp.Result.AccountData.SignerLists), 1)
	})
}
