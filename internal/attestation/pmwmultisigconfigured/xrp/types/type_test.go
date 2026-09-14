package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSignerListsV1(t *testing.T) {
	raw := `{
		"result": {
			"status": "success",
			"account_data": {
				"Account": "rTest",
				"Sequence": 1,
				"signer_lists": [{"SignerQuorum": 2, "SignerEntries": []}]
			},
			"account_flags": {}
		}
	}`
	var resp AccountInfoResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))

	signers := resp.Result.ResolveSignerLists()
	require.Len(t, signers, 1)
	require.Equal(t, uint64(2), signers[0].SignerQuorum)
}

func TestResolveSignerListsV2Clio(t *testing.T) {
	raw := `{
		"result": {
			"status": "success",
			"account_data": {
				"Account": "rTest",
				"Sequence": 1
			},
			"account_flags": {},
			"signer_lists": [{"SignerQuorum": 3, "SignerEntries": []}]
		}
	}`
	var resp AccountInfoResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))

	signers := resp.Result.ResolveSignerLists()
	require.Len(t, signers, 1)
	require.Equal(t, uint64(3), signers[0].SignerQuorum)
}

func TestNetworkIDUnmarshalJSON(t *testing.T) {
	t.Run("absent values leave Present false", func(t *testing.T) {
		for name, raw := range map[string]string{
			"null":         `null`,
			"empty string": `""`,
		} {
			t.Run(name, func(t *testing.T) {
				var n NetworkID
				require.NoError(t, json.Unmarshal([]byte(raw), &n))
				require.False(t, n.Present)
				require.Zero(t, n.Value)
			})
		}
	})
	t.Run("invalid values are rejected", func(t *testing.T) {
		for name, raw := range map[string]string{
			"non-numeric":     `"mainnet"`,
			"beyond uint32":   `4294967296`,
			"negative number": `-1`,
		} {
			t.Run(name, func(t *testing.T) {
				var n NetworkID
				require.ErrorContains(t, json.Unmarshal([]byte(raw), &n), "invalid network_id")
				require.False(t, n.Present)
			})
		}
	})
	t.Run("number and quoted string parse alike", func(t *testing.T) {
		for name, raw := range map[string]string{
			"rippled number": `1`,
			"Clio string":    `"1"`,
		} {
			t.Run(name, func(t *testing.T) {
				var n NetworkID
				require.NoError(t, json.Unmarshal([]byte(raw), &n))
				require.True(t, n.Present)
				require.Equal(t, uint32(1), n.Value)
			})
		}
	})
}

func TestResolveSignerListsEmpty(t *testing.T) {
	raw := `{
		"result": {
			"status": "success",
			"account_data": {"Account": "rTest", "Sequence": 1},
			"account_flags": {}
		}
	}`
	var resp AccountInfoResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))

	signers := resp.Result.ResolveSignerLists()
	require.Empty(t, signers)
}
