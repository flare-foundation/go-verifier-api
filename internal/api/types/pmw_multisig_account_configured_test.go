package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestToInternalRejectsTooManyPublicKeys(t *testing.T) {
	keys := make([]hexutil.Bytes, maxPublicKeys+1)
	for i := range keys {
		keys[i] = hexutil.Bytes{byte(i), 0x02}
	}
	req := PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: "rTest",
		PublicKeys:     keys,
		Threshold:      1,
	}
	_, err := req.ToInternal()
	require.ErrorContains(t, err, "too many public keys")
}

func TestToInternalRejectsEmptyPublicKey(t *testing.T) {
	req := PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: "rTest",
		PublicKeys:     []hexutil.Bytes{{}},
		Threshold:      1,
	}
	_, err := req.ToInternal()
	require.ErrorContains(t, err, "public key at index 0 is empty")
}

func TestToInternalAcceptsValid(t *testing.T) {
	req := PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: "rTest",
		PublicKeys:     []hexutil.Bytes{{0x01, 0x02}, {0x03, 0x04}},
		Threshold:      1,
	}
	out, err := req.ToInternal()
	require.NoError(t, err)
	require.Equal(t, "rTest", out.AccountAddress)
	require.Len(t, out.PublicKeys, 2)
	require.Equal(t, uint64(1), out.Threshold)
}
