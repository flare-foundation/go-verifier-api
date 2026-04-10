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
