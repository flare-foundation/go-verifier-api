package config

import (
	"encoding/pem"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/connector"
	"github.com/stretchr/testify/require"
)

func TestBuildTeeAvailabilityCheckConfigError(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:        SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: RELAY_CONTRACT_ADDRESS, TEE_MACHINE_REGISTRY_CONTRACT_ADDRESS, RPC_URL")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                          SourceTEE,
			AttestationType:                   "UnknownType",
			RelayContractAddress:              "URL",
			TeeMachineRegistryContractAddress: "URL",
			RPCURL:                            "URL",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
}

func TestBuildTeeAvailabilityCheckConfigSuccess(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                          SourceTEE,
			AttestationType:                   connector.AvailabilityCheck,
			RelayContractAddress:              "0x1234",
			TeeMachineRegistryContractAddress: "0x5678",
			RPCURL:                            "https://rpc.example.com",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.False(t, cfg.AllowTeeDebug)
		require.False(t, cfg.DisableAttestationCheckE2E)
		require.False(t, cfg.AllowPrivateNetworks)
		require.Equal(t, "0x1234", cfg.RelayContractAddress)
		require.Equal(t, "0x5678", cfg.TeeMachineRegistryContractAddress)
		require.Equal(t, "https://rpc.example.com", cfg.RPCURL)
		require.NotNil(t, cfg.GoogleRootCertificate)
	})
	t.Run("allow private networks enabled", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                          SourceTEE,
			AttestationType:                   connector.AvailabilityCheck,
			RelayContractAddress:              "0x1234",
			TeeMachineRegistryContractAddress: "0x5678",
			RPCURL:                            "https://rpc.example.com",
			AllowPrivateNetworks:              "true",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.True(t, cfg.AllowPrivateNetworks)
	})
	t.Run("all flags enabled", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                          SourceTEE,
			AttestationType:                   connector.AvailabilityCheck,
			RelayContractAddress:              "0x1234",
			TeeMachineRegistryContractAddress: "0x5678",
			RPCURL:                            "https://rpc.example.com",
			AllowTeeDebug:                     "true",
			DisableAttestationCheckE2E:        "true",
			AllowPrivateNetworks:              "true",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.True(t, cfg.AllowTeeDebug)
		require.True(t, cfg.DisableAttestationCheckE2E)
		require.True(t, cfg.AllowPrivateNetworks)
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
