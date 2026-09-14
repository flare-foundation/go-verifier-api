package api

import (
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParseSourceID(t *testing.T) {
	for _, sid := range SourceIDs {
		t.Run(string(sid), func(t *testing.T) {
			got, err := parseSourceID(string(sid))
			require.NoError(t, err)
			require.Equal(t, sid, got)
		})
	}
	t.Run("invalid-source", func(t *testing.T) {
		_, err := parseSourceID("invalid-source")
		require.ErrorContains(t, err, "invalid source id")
	})
}

func TestGetAPIKeys(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantKeys  []string
		wantError string
	}{
		{"unset", "", nil, "API_KEYS must be set"},
		{"empty string", "   ", nil, "API_KEYS must be set"},
		{"only empty values", " , , ", nil, "API_KEYS contains only empty values"},
		{"key too short", "shortkey", nil, "must be at least 16 characters"},
		{"one key too short among valid", "0123456789abcdef,shortkey", nil, "must be at least 16 characters"},
		{"trailing comma", "0123456789abcdef,fedcba9876543210,", []string{"0123456789abcdef", "fedcba9876543210"}, ""},
		{"single key", "0123456789abcdef", []string{"0123456789abcdef"}, ""},
		{"multiple keys with spaces", "0123456789abcdef, fedcba9876543210 ,abcdef0123456789", []string{"0123456789abcdef", "fedcba9876543210", "abcdef0123456789"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.EnvAPIKeys, tt.envValue)
			keys, err := getAPIKeys()
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Nil(t, keys)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantKeys, keys)
			}
		})
	}
}

func TestGetEnvOrError(t *testing.T) {
	const testKey = "API_KEYS"
	tests := []struct {
		name      string
		envValue  string
		wantValue string
		wantError string
	}{
		{"unset", "", "", testKey + " must be set"},
		{"empty string", "   ", "", testKey + " must be set"},
		{"valid value", "value", "value", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(testKey, tt.envValue)
			val, err := getEnvOrError(testKey)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Equal(t, "", val)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantValue, val)
			}
		})
	}
}

func TestLoadEnvConfig(t *testing.T) {
	t.Run("Env config should load only with all required fields", func(t *testing.T) {
		loadEnvShouldFail(t)
		t.Setenv(config.EnvPort, "1234")
		loadEnvShouldFail(t)
		t.Setenv(config.EnvSourceID, string(config.SourceTEE))
		loadEnvShouldFail(t)
		t.Setenv(config.EnvAPIKeys, "0123456789abcdef,fedcba9876543210")
		t.Setenv(config.EnvAllowPrivateNetworks, "true")

		cfg, err := LoadEnvConfig()
		require.NoError(t, err)
		require.Equal(t, "1234", cfg.Port)
		// The source alone selects the served types: TEE serves TeeAvailabilityCheck.
		require.Equal(t, config.SourceAttestationTypes[config.SourceTEE], cfg.ServedAttestationTypes())
		require.Equal(t, config.SourceTEE, cfg.SourceID)
		require.Equal(t, "true", cfg.AllowPrivateNetworks)
	})
	t.Run("Env config serves all attestation types for the source", func(t *testing.T) {
		t.Setenv(config.EnvPort, "1234")
		t.Setenv(config.EnvSourceID, string(config.SourceXRP))
		t.Setenv(config.EnvAPIKeys, "0123456789abcdef")

		cfg, err := LoadEnvConfig()
		require.NoError(t, err)
		require.Equal(t, config.SourceAttestationTypes[config.SourceXRP], cfg.ServedAttestationTypes())
	})
	t.Run("Env config should fail if source id is invalid", func(t *testing.T) {
		t.Setenv(config.EnvPort, "1234")
		t.Setenv(config.EnvSourceID, "invalid-source-id")
		_, err := LoadEnvConfig()
		require.ErrorContains(t, err, "invalid source id: invalid-source-id")
	})
}

func loadEnvShouldFail(t *testing.T) {
	t.Helper()
	_, err := LoadEnvConfig()
	require.ErrorContains(t, err, "must be set")
}

func TestResolveAttestationTypes(t *testing.T) {
	t.Run("serves all types for a known source", func(t *testing.T) {
		got, err := resolveAttestationTypes(config.SourceXRP)
		require.NoError(t, err)
		require.Equal(t, config.SourceAttestationTypes[config.SourceXRP], got)
	})
	t.Run("serves the single type of the TEE source", func(t *testing.T) {
		got, err := resolveAttestationTypes(config.SourceTEE)
		require.NoError(t, err)
		require.Equal(t, []fdc2.AttestationType{fdc2.AvailabilityCheck}, got)
	})
	t.Run("rejects an unknown source", func(t *testing.T) {
		_, err := resolveAttestationTypes(config.SourceName("nope"))
		require.ErrorContains(t, err, "no attestation types defined for source")
	})
}

// TestSourceAttestationTypesAreRegisterable guards the coupling between the
// source→types map and LoadModule's switch: every type a source advertises must
// be one registerVerifier can actually construct. Missing config is expected here
// (we pass none); an "unsupported attestation type" would mean the map lists a
// type LoadModule cannot register.
func TestSourceAttestationTypesAreRegisterable(t *testing.T) {
	config.ClearPMWPaymentStatusConfigForTest()
	config.ClearPMWFeeProofConfigForTest()
	config.ClearPMWMultisigAccountConfiguredConfigForTest()
	config.ClearTeeAvailabilityCheckConfigForTest()

	apiInst := huma.NewAPI(huma.DefaultConfig("test", "0.0.0"), mockAdapter{})
	for _, src := range SourceIDs {
		types, ok := config.AttestationTypesForSource(src)
		require.Truef(t, ok, "source %s has no attestation types mapping", src)
		require.NotEmpty(t, types)
		for _, at := range types {
			cfg := config.EnvConfig{SourceID: src, AttestationType: at}
			if _, err := registerVerifier(apiInst, cfg); err != nil {
				require.NotContainsf(t, err.Error(), "unsupported attestation type",
					"source %s maps type %s that registerVerifier cannot handle", src, at)
			}
		}
	}
}
