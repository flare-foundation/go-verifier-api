package api

import (
	"fmt"
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
	require.ErrorContains(t, err, "invalid attestation type")
}

func TestParseSourceID(t *testing.T) {
	// Valid source IDs
	for _, sid := range SourceIDs {
		got, err := parseSourceID(string(sid))
		require.NoError(t, err)
		require.Equal(t, sid, got)
	}
	// Invalid source ID
	_, err := parseSourceID("invalid-source")
	require.ErrorContains(t, err, "invalid source id")
}

func TestGetAPIKeys(t *testing.T) {
	keys, err := getAPIKeys()
	require.ErrorContains(t, err, "API_KEYS must be set")
	require.Nil(t, keys)
	// Empty string
	t.Setenv(config.EnvAPIKeys, "   ")
	keys, err = getAPIKeys()
	require.ErrorContains(t, err, "API_KEYS must be set")
	require.Nil(t, keys)
	// Only empty values
	t.Setenv(config.EnvAPIKeys, " , , ")
	keys, err = getAPIKeys()
	require.ErrorContains(t, err, "API_KEYS contains only empty values")
	require.Nil(t, keys)
	// Trailing comma key
	t.Setenv(config.EnvAPIKeys, "key1,key2,")
	keys, err = getAPIKeys()
	require.NoError(t, err)
	require.Equal(t, []string{"key1", "key2"}, keys)
	// Single key
	t.Setenv(config.EnvAPIKeys, "key1")
	keys, err = getAPIKeys()
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

	val, err := getEnvOrError(testKey)
	require.ErrorContains(t, err, fmt.Sprintf("%s must be set", testKey))
	require.Equal(t, "", val)

	t.Setenv(testKey, "   ")
	val, err = getEnvOrError(testKey)
	require.ErrorContains(t, err, fmt.Sprintf("%s must be set", testKey))
	require.Equal(t, "", val)

	t.Setenv(testKey, "value")
	val, err = getEnvOrError(testKey)
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
	_, err := LoadEnvConfig()
	require.ErrorContains(t, err, "must be set")
}
