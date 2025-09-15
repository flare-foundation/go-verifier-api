package api

import (
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParseAttestationType(t *testing.T) {
	// Valid types
	for _, at := range []connector.AttestationType{
		connector.AvailabilityCheck,
		connector.PMWPaymentStatus,
		connector.PMWMultisigAccountConfigured,
	} {
		got, err := parseAttestationType(string(at))
		require.NoError(t, err)
		require.Equal(t, at, got)
	}

	// Invalid type
	_, err := parseAttestationType("invalid-type")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid attestation type")
}

func TestParseSourceID(t *testing.T) {
	// Valid source IDs
	for _, sid := range []config.SourceName{
		config.SourceTEE,
		config.SourceXRP,
	} {
		got, err := parseSourceID(string(sid))
		require.NoError(t, err)
		require.Equal(t, sid, got)
	}

	// Invalid source ID
	_, err := parseSourceID("invalid-source")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid source id")
}

func TestGetAPIKeys(t *testing.T) {
	_, err := getAPIKeys()
	require.Error(t, err)

	// Empty string
	t.Setenv(config.EnvAPIKeys, "   ")
	_, err = getAPIKeys()
	require.Error(t, err)

	// Only empty values
	t.Setenv(config.EnvAPIKeys, " , , ")
	_, err = getAPIKeys()
	require.Error(t, err)

	// Single key
	t.Setenv(config.EnvAPIKeys, "key1")
	keys, err := getAPIKeys()
	require.NoError(t, err)
	require.Equal(t, []string{"key1"}, keys)

	// Multiple keys, with spaces
	t.Setenv(config.EnvAPIKeys, "key1, key2 ,key3")
	keys, err = getAPIKeys()
	require.NoError(t, err)
	require.Equal(t, []string{"key1", "key2", "key3"}, keys)
}

func TestGetEnvOrError(t *testing.T) {
	const testKey = "API_KEYS"

	_, err := getEnvOrError(testKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), testKey)

	t.Setenv(testKey, "   ")
	_, err = getEnvOrError(testKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), testKey)

	t.Setenv(testKey, "value")
	val, err := getEnvOrError(testKey)
	require.NoError(t, err)
	require.Equal(t, "value", val)
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
		loadEnvShouldFail(t)
		t.Setenv(config.EnvAllowTeeDebug, "false")
		loadEnvShouldFail(t)
		t.Setenv(config.EnvDisableAttestationCheckE2E, "false")

		cfg, err := LoadEnvConfig()
		require.NoError(t, err)
		require.Equal(t, "1234", cfg.Port)
		require.Equal(t, connector.AvailabilityCheck, cfg.AttestationType)
		require.Equal(t, config.SourceTEE, cfg.SourceID)
	})

	t.Run("Env config should fail if attestation type is invalid", func(t *testing.T) {
		t.Setenv(config.EnvPort, "1234")
		t.Setenv(config.EnvSourceID, string(config.SourceTEE))
		t.Setenv(config.EnvAttestationType, "invalid-attestation-type")
		_, err := LoadEnvConfig()
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid attestation type")
	})

	t.Run("Env config should fail if source id is invalid", func(t *testing.T) {
		t.Setenv(config.EnvPort, "1234")
		t.Setenv(config.EnvSourceID, "invalid-source-id")
		t.Setenv(config.EnvAttestationType, string(connector.AvailabilityCheck))
		_, err := LoadEnvConfig()
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid source id")
	})
}

func loadEnvShouldFail(t *testing.T) {
	_, err := LoadEnvConfig()
	require.Error(t, err)
}
