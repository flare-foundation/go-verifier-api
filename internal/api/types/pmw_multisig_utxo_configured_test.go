package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func key(n int) hexutil.Bytes {
	b := make([]byte, 78)
	for i := range b {
		b[i] = byte(n)
	}
	return b
}

func anchor(b byte, vout uint32) Anchor {
	txid := make([]byte, 32)
	for i := range txid {
		txid[i] = b
	}
	return Anchor{GenesisAnchorTxid: txid, GenesisAnchorVout: vout}
}

func validBody() PMWMultisigUtxoConfiguredRequestBody {
	return PMWMultisigUtxoConfiguredRequestBody{
		AccountIndex: 0,
		PublicKeys:   []hexutil.Bytes{key(1), key(2), key(3)},
		Threshold:    2,
		Anchors:      []Anchor{anchor(1, 0), anchor(2, 1)},
	}
}

func TestToInternalValid(t *testing.T) {
	internal, err := validBody().ToInternal()
	require.NoError(t, err)
	require.Equal(t, uint32(0), internal.AccountIndex)
	require.Equal(t, uint64(2), internal.Threshold)
	require.Len(t, internal.PublicKeys, 3)
	require.Len(t, internal.Anchors, 2)
	require.Equal(t, byte(2), internal.Anchors[1].GenesisAnchorTxid[0])
	require.Equal(t, uint32(1), internal.Anchors[1].GenesisAnchorVout)
}

func TestToInternalRejects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PMWMultisigUtxoConfiguredRequestBody)
	}{
		{"emptyKeys", func(b *PMWMultisigUtxoConfiguredRequestBody) { b.PublicKeys = nil }},
		{"thresholdZero", func(b *PMWMultisigUtxoConfiguredRequestBody) { b.Threshold = 0 }},
		{"thresholdAboveN", func(b *PMWMultisigUtxoConfiguredRequestBody) { b.Threshold = 4 }},
		{"emptyAnchors", func(b *PMWMultisigUtxoConfiguredRequestBody) { b.Anchors = nil }},
		{"shortTxid", func(b *PMWMultisigUtxoConfiguredRequestBody) {
			b.Anchors[0].GenesisAnchorTxid = []byte{0x01, 0x02}
		}},
		{"tooManyKeys", func(b *PMWMultisigUtxoConfiguredRequestBody) {
			keys := make([]hexutil.Bytes, 21)
			for i := range keys {
				keys[i] = key(i)
			}
			b.PublicKeys = keys
		}},
		{"emptyIndividualKey", func(b *PMWMultisigUtxoConfiguredRequestBody) {
			b.PublicKeys[1] = hexutil.Bytes{}
		}},
		{"duplicateKeys", func(b *PMWMultisigUtxoConfiguredRequestBody) {
			b.PublicKeys[2] = b.PublicKeys[0]
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := validBody()
			tc.mutate(&b)
			_, err := b.ToInternal()
			require.Error(t, err)
		})
	}
}

// TestLogHelpers exercises the debug logging helpers for the request and
// response bodies — they are pure side-effect calls, so the test only asserts
// they do not panic on well-formed input.
func TestLogHelpers(t *testing.T) {
	internal, err := validBody().ToInternal()
	require.NoError(t, err)
	require.NotPanics(t, func() { LogPMWMultisigUtxoConfiguredRequestBody(internal) })

	resp := PMWMultisigUtxoConfiguredResponseBody{Status: 0, AccountAddress: "bc1qexample"}
	require.NotPanics(t, resp.Log)
}
