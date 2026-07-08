package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestToInternalRejectsTooManyPublicKeys(t *testing.T) {
	keys := make([]hexutil.Bytes, MaxPublicKeys+1)
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

func TestValidatePublicKeys(t *testing.T) {
	t.Run("nil slice is valid", func(t *testing.T) {
		require.NoError(t, ValidatePublicKeys(nil))
	})
	t.Run("empty slice is valid", func(t *testing.T) {
		require.NoError(t, ValidatePublicKeys([][]byte{}))
	})
	t.Run("at cap is valid", func(t *testing.T) {
		keys := make([][]byte, MaxPublicKeys)
		for i := range keys {
			keys[i] = []byte{byte(i), 0x02}
		}
		require.NoError(t, ValidatePublicKeys(keys))
	})
	t.Run("over cap is rejected", func(t *testing.T) {
		keys := make([][]byte, MaxPublicKeys+1)
		for i := range keys {
			keys[i] = []byte{byte(i), 0x02}
		}
		err := ValidatePublicKeys(keys)
		require.ErrorContains(t, err, "too many public keys")
	})
	t.Run("empty entry is rejected", func(t *testing.T) {
		err := ValidatePublicKeys([][]byte{{0x01}, nil, {0x03}})
		require.ErrorContains(t, err, "public key at index 1 is empty")
	})
	t.Run("duplicate entry is rejected", func(t *testing.T) {
		err := ValidatePublicKeys([][]byte{{0x01, 0x02}, {0x03, 0x04}, {0x01, 0x02}})
		require.ErrorContains(t, err, "duplicates index 0")
	})
}

func TestValidateMultisigRequest(t *testing.T) {
	validKeys := [][]byte{{0x01, 0x02}, {0x03, 0x04}}

	t.Run("valid request", func(t *testing.T) {
		require.NoError(t, ValidateMultisigRequest(validKeys, 2))
	})
	t.Run("empty public-key list rejected", func(t *testing.T) {
		require.ErrorContains(t, ValidateMultisigRequest([][]byte{}, 1), "publicKeys must not be empty")
	})
	t.Run("nil public-key list rejected", func(t *testing.T) {
		require.ErrorContains(t, ValidateMultisigRequest(nil, 1), "publicKeys must not be empty")
	})
	t.Run("zero threshold rejected", func(t *testing.T) {
		require.ErrorContains(t, ValidateMultisigRequest(validKeys, 0), "threshold must be greater than zero")
	})
	t.Run("delegates per-key checks", func(t *testing.T) {
		require.ErrorContains(t, ValidateMultisigRequest([][]byte{{0x01}, nil}, 1), "public key at index 1 is empty")
	})
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

func TestToInternalRejectsZeroThreshold(t *testing.T) {
	req := PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: "rTest",
		PublicKeys:     []hexutil.Bytes{{0x01, 0x02}},
		Threshold:      0,
	}
	_, err := req.ToInternal()
	require.ErrorContains(t, err, "threshold must be greater than zero")
}

func TestToInternalRejectsEmptyPublicKeyList(t *testing.T) {
	req := PMWMultisigAccountConfiguredRequestBody{
		AccountAddress: "rTest",
		PublicKeys:     nil,
		Threshold:      1,
	}
	_, err := req.ToInternal()
	require.ErrorContains(t, err, "publicKeys must not be empty")
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
