package transaction

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetStringField(t *testing.T) {
	m := map[string]interface{}{
		"key1": "val1",
		"key2": 1234,
	}
	t.Run("valid string field", func(t *testing.T) {
		val, ok := getStringField(m, "key1")
		require.True(t, ok)
		require.Equal(t, "val1", val)
	})
	t.Run("number field", func(t *testing.T) {
		_, ok := getStringField(m, "key2")
		require.False(t, ok)
	})
	t.Run("missing field", func(t *testing.T) {
		_, ok := getStringField(m, "missing")
		require.False(t, ok)
	})
}
