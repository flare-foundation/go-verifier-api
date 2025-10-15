package config_test

import (
	"testing"

	pmwpaymentstatusconfig "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/config"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadPMWPaymentStatusConfigError(t *testing.T) {
	t.Run("missing variable", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:        config.SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := pmwpaymentstatusconfig.LoadPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: CCHAIN_DATABASE_URL, SOURCE_DATABASE_URL")
	})
	t.Run("missing variable", func(t *testing.T) {
		envConfig := config.EnvConfig{
			SourceID:          config.SourceTEE,
			AttestationType:   "UnknownType",
			SourceDatabaseURL: "URL",
			CChainDatabaseURL: "URL",
		}
		cfg, err := pmwpaymentstatusconfig.LoadPMWPaymentStatusConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}
