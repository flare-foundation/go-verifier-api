package config_test

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestRPCURLRouting pins the split introduced for per-source deployments: with
// both SOURCE_RPC_URL and FLARE_RPC_URL set to distinct values, each config must
// pick the right one — Multisig the source node, the others the Flare node — so a
// future edit cannot cross-wire them.
func TestRPCURLRouting(t *testing.T) {
	const (
		sourceRPC = "http://source-node:51234"
		flareRPC  = "http://flare-node:8545"
	)
	base := config.EnvConfig{
		DestinationChainURLSlug:        "coston",
		SourceRPCURL:                   sourceRPC,
		FlareRPCURL:                    flareRPC,
		SourceDatabaseURL:              "postgres://localhost/test",
		CChainDatabaseURL:              "root:root@tcp(localhost)/db",
		FlareTeeManagerContractAddress: "0x00000000000000000000000000000000000000C1",
		TeePaymentsContractAddress:     "0x00000000000000000000000000000000000000C2",
		RelayContractAddress:           "0x0000000000000000000000000000000000000001",
		SourceID:                       config.SourceTestXRP,
		ChainID:                        "16",
		TeeAudience:                    "aud",
	}

	t.Run("Multisig uses the source RPC", func(t *testing.T) {
		cfg := base
		cfg.AttestationType = fdc2.PMWMultisigAccountConfigured
		got, err := config.BuildPMWMultisigAccountConfiguredConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, sourceRPC, got.SourceRPCURL)
	})
	t.Run("PaymentStatus uses the Flare RPC", func(t *testing.T) {
		cfg := base
		cfg.AttestationType = fdc2.PMWPaymentStatus
		got, err := config.BuildPMWPaymentStatusConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, flareRPC, got.FlareRPCURL)
	})
	t.Run("FeeProof uses the Flare RPC", func(t *testing.T) {
		cfg := base
		cfg.AttestationType = fdc2.PMWFeeProof
		got, err := config.BuildPMWFeeProofConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, flareRPC, got.FlareRPCURL)
	})
	t.Run("TeeAvailabilityCheck uses the Flare RPC", func(t *testing.T) {
		cfg := base
		cfg.SourceID = config.SourceTEE
		cfg.AttestationType = fdc2.AvailabilityCheck
		got, err := config.BuildTeeAvailabilityCheckConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, flareRPC, got.FlareRPCURL)
	})
}

// TestServedAttestationTypes covers the resolution used by LoadModule: an
// explicit list wins, otherwise the single type, otherwise nothing.
func TestServedAttestationTypes(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.EnvConfig
		want []fdc2.AttestationType
	}{
		{
			name: "list takes precedence over the singular type",
			cfg:  config.EnvConfig{AttestationTypes: []fdc2.AttestationType{fdc2.PMWFeeProof}, AttestationType: fdc2.PMWPaymentStatus},
			want: []fdc2.AttestationType{fdc2.PMWFeeProof},
		},
		{
			name: "falls back to the singular type",
			cfg:  config.EnvConfig{AttestationType: fdc2.PMWPaymentStatus},
			want: []fdc2.AttestationType{fdc2.PMWPaymentStatus},
		},
		{
			name: "empty when neither is set",
			cfg:  config.EnvConfig{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.cfg.ServedAttestationTypes())
		})
	}
}
