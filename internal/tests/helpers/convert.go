package helpers

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"

	"github.com/stretchr/testify/require"
)

func EncodeRequestBody[T any](t *testing.T, attType fdc2.AttestationType, body T) []byte {
	t.Helper()
	result, err := structs.Encode(fdc2.AttestationTypeArguments[attType].Request, body)
	require.NoError(t, err)
	return result
}

func DecodeResponseBody[T any](t *testing.T, attType fdc2.AttestationType, data []byte) T {
	t.Helper()
	var resp T
	err := structs.DecodeTo(fdc2.AttestationTypeArguments[attType].Response, data, &resp)
	require.NoError(t, err)
	return resp
}
