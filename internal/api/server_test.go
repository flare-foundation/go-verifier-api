package api

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParseAttestationType(t *testing.T) {
	for _, at := range AttestationTypes {
		t.Run(string(at), func(t *testing.T) {
			got, err := parseAttestationType(string(at))
			require.NoError(t, err)
			require.Equal(t, at, got)
		})
	}
	t.Run("invalid-source", func(t *testing.T) {
		_, err := parseAttestationType("invalid-type")
		require.ErrorContains(t, err, "invalid attestation type")
	})
}

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
		{"trailing comma", "key1,key2,", []string{"key1", "key2"}, ""},
		{"single key", "key1", []string{"key1"}, ""},
		{"multiple keys with spaces", "key1, key2 ,key3", []string{"key1", "key2", "key3"}, ""},
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
		t.Setenv(config.EnvAttestationType, string(connector.AvailabilityCheck))
		loadEnvShouldFail(t)
		t.Setenv(config.EnvSourceID, string(config.SourceTEE))
		loadEnvShouldFail(t)
		t.Setenv(config.EnvAPIKeys, "key1,key2")
		t.Setenv(config.EnvAllowPrivateNetworks, "true")

		cfg, err := LoadEnvConfig()
		require.NoError(t, err)
		require.Equal(t, "1234", cfg.Port)
		require.Equal(t, connector.AvailabilityCheck, cfg.AttestationType)
		require.Equal(t, config.SourceTEE, cfg.SourceID)
		require.Equal(t, "true", cfg.AllowPrivateNetworks)
	})
	t.Run("Env config should fail if attestation type is invalid", func(t *testing.T) {
		t.Setenv(config.EnvPort, "1234")
		t.Setenv(config.EnvSourceID, string(config.SourceTEE))
		t.Setenv(config.EnvAttestationType, "invalid-attestation-type")
		_, err := LoadEnvConfig()
		require.ErrorContains(t, err, "invalid attestation type: invalid-attestation-type")
	})
	t.Run("Env config should fail if source id is invalid", func(t *testing.T) {
		t.Setenv(config.EnvPort, "1234")
		t.Setenv(config.EnvSourceID, "invalid-source-id")
		t.Setenv(config.EnvAttestationType, string(connector.AvailabilityCheck))
		_, err := LoadEnvConfig()
		require.ErrorContains(t, err, "invalid source id: invalid-source-id")
	})
}

func loadEnvShouldFail(t *testing.T) {
	t.Helper()
	_, err := LoadEnvConfig()
	require.ErrorContains(t, err, "must be set")
}
