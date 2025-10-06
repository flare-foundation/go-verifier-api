package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

const testAddress = "rMDCrSYbeGm77aYjnvuHVnBwZ1TkLnu1UL"

func TestGetAccountInfo(t *testing.T) {
	t.Run("invalid rpc url", func(t *testing.T) {
		client := NewClient("https://invalid.invalid")
		ctx := context.Background()
		val, err := client.GetAccountInfo(ctx, testAddress)
		require.ErrorContains(t, err, "cannot get account info")
		require.ErrorContains(t, err, "HTTP request failed for")
		require.ErrorContains(t, err, "no such host")
		require.Nil(t, val)
	})
	t.Run("invalid address", func(t *testing.T) {
		client := NewClient("https://s.altnet.rippletest.net:51234")
		ctx := context.Background()
		val, err := client.GetAccountInfo(ctx, "0x")
		require.ErrorContains(t, err, "XRP RPC returned non-success status")
		require.Nil(t, val)
	})
	t.Run("valid address", func(t *testing.T) {
		client := NewClient("https://s.altnet.rippletest.net:51234")
		ctx := context.Background()
		resp, err := client.GetAccountInfo(ctx, testAddress)
		require.NoError(t, err)
		require.Equal(t, testAddress, resp.Result.AccountData.Account)
		require.NotZero(t, resp.Result.AccountData.Sequence)
		require.GreaterOrEqual(t, len(resp.Result.AccountData.SignerLists), 1)
	})
}
