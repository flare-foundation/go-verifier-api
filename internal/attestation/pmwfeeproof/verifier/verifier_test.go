package verifier

import (
	"testing"

	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestConstructorForSource(t *testing.T) {
	t.Run("supported sources resolve", func(t *testing.T) {
		for _, src := range []string{string(config.SourceXRP), string(config.SourceTestXRP)} {
			c, err := ConstructorForSource(src)
			require.NoError(t, err)
			require.NotNil(t, c)
		}
	})
	t.Run("unsupported source errors", func(t *testing.T) {
		for _, src := range []string{"UNSUPPORTED_SOURCE", "", string(config.SourceTEE)} {
			c, err := ConstructorForSource(src)
			require.Nil(t, c)
			require.ErrorContains(t, err, "no verifier for sourceID")
		}
	})
}
