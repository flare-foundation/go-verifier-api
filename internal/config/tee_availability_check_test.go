package config

import (
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadPMWPaymentStatusConfigError(t *testing.T) {
	t.Run("missing variable", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:        SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := LoadTeeAvailabilityCheckConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: RELAY_CONTRACT_ADDRESS, TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS, RPC_URL")
	})
	t.Run("missing variable", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                          SourceTEE,
			AttestationType:                   "UnknownType",
			RelayContractAddress:              "URL",
			TeeMachineRegistryContractAddress: "URL",
			RPCURL:                            "URL",
		}
		cfg, err := LoadTeeAvailabilityCheckConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}
func TestGetBoolOrSetFalse(t *testing.T) {
	t.Run("empty value", func(t *testing.T) {
		res := getBoolOrSetFalse("KEY", "")
		require.False(t, res)
	})
	t.Run("not bool", func(t *testing.T) {
		res := getBoolOrSetFalse("KEY", "fals")
		require.False(t, res)
	})
	t.Run("empty value", func(t *testing.T) {
		res := getBoolOrSetFalse("KEY", "")
		require.False(t, res)
	})
	t.Run("valid", func(t *testing.T) {
		res := getBoolOrSetFalse("KEY", "true")
		require.True(t, res)
		res = getBoolOrSetFalse("KEY", "false")
		require.False(t, res)
	})
}

func TestLoadGoogleRootCert(t *testing.T) {
	t.Run("invalid cert", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
		_, err := loadGoogleRootCertFromBytes(badPEM)
		require.ErrorContains(t, err, "cannot parse embedded Google root certificate")
	})
	t.Run("invalid PEM", func(t *testing.T) {
		_, err := loadGoogleRootCertFromBytes([]byte("not-a-pem"))
		require.ErrorContains(t, err, "invalid PEM format")
	})
}
