package testhelper

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"

	"github.com/stretchr/testify/require"
)

func EncodeRequestBody[T any](t *testing.T, attType connector.AttestationType, body T) []byte {
	t.Helper()
	result, err := structs.Encode(connector.AttestationTypeArguments[attType].Request, body)
	require.NoError(t, err)
	return result
}

func DecodeResponseBody[T any](t *testing.T, attType connector.AttestationType, data []byte) T {
	t.Helper()
	var resp T
	err := structs.DecodeTo(connector.AttestationTypeArguments[attType].Response, data, &resp)
	require.NoError(t, err)
	return resp
}

func ToHexutilBytesSlice(src [][]byte) []hexutil.Bytes {
	res := make([]hexutil.Bytes, len(src))
	for i, b := range src {
		res[i] = hexutil.Bytes(b)
	}
	return res
}
