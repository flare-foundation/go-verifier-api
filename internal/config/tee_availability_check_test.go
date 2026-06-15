package config

import (
	"encoding/pem"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
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
		require.ErrorContains(t, err, "missing environment variables: RELAY_CONTRACT_ADDRESS, RPC_URL")
	})
	t.Run("invalid RELAY_CONTRACT_ADDRESS hex", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:             SourceTEE,
			AttestationType:      "UnknownType",
			RelayContractAddress: "not-hex",
			RPCURL:               "URL",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "RELAY_CONTRACT_ADDRESS is not a valid hex address")
	})
	t.Run("invalid attestation type", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:             SourceTEE,
			AttestationType:      "UnknownType",
			RelayContractAddress: "0x0000000000000000000000000000000000000001",
			RPCURL:               "URL",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "no ABI struct names defined for attestation type UnknownType")
	})
	boolFlagCases := []struct {
		name    string
		mutate  func(*EnvConfig)
		wantErr string
	}{
		{
			name:    "invalid ALLOW_TEE_DEBUG fails the boot",
			mutate:  func(c *EnvConfig) { c.AllowTeeDebug = "ture" },
			wantErr: `ALLOW_TEE_DEBUG has invalid bool value "ture"`,
		},
		{
			name:    "invalid DISABLE_ATTESTATION_CHECK_E2E fails the boot",
			mutate:  func(c *EnvConfig) { c.DisableAttestationCheckE2E = "nope" },
			wantErr: `DISABLE_ATTESTATION_CHECK_E2E has invalid bool value "nope"`,
		},
		{
			name:    "invalid ALLOW_PRIVATE_NETWORKS fails the boot",
			mutate:  func(c *EnvConfig) { c.AllowPrivateNetworks = "yes" },
			wantErr: `ALLOW_PRIVATE_NETWORKS has invalid bool value "yes"`,
		},
	}
	for _, tc := range boolFlagCases {
		t.Run(tc.name, func(t *testing.T) {
			envConfig := EnvConfig{
				SourceID:             SourceTEE,
				AttestationType:      fdc2.AvailabilityCheck,
				RelayContractAddress: "0x0000000000000000000000000000000000000001",
				RPCURL:               "https://rpc.example.com",
			}
			tc.mutate(&envConfig)
			cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
			require.Nil(t, cfg)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestBuildTeeAvailabilityCheckConfigSuccess(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:             SourceTEE,
			AttestationType:      fdc2.AvailabilityCheck,
			RelayContractAddress: "0x0000000000000000000000000000000000000001",
			RPCURL:               "https://rpc.example.com",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.False(t, cfg.AllowTeeDebug)
		require.False(t, cfg.DisableAttestationCheckE2E)
		require.False(t, cfg.AllowPrivateNetworks)
		require.NotEqual(t, cfg.RelayContractAddress, [20]byte{})
		require.Equal(t, "https://rpc.example.com", cfg.RPCURL)
		require.NotNil(t, cfg.GoogleRootCertificate)
	})
	t.Run("allow private networks enabled", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:             SourceTEE,
			AttestationType:      fdc2.AvailabilityCheck,
			RelayContractAddress: "0x0000000000000000000000000000000000000001",
			RPCURL:               "https://rpc.example.com",
			AllowPrivateNetworks: "true",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.True(t, cfg.AllowPrivateNetworks)
	})
	t.Run("all flags enabled", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                   SourceTEE,
			AttestationType:            fdc2.AvailabilityCheck,
			RelayContractAddress:       "0x0000000000000000000000000000000000000001",
			RPCURL:                     "https://rpc.example.com",
			AllowTeeDebug:              "true",
			DisableAttestationCheckE2E: "true",
			AllowPrivateNetworks:       "true",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.True(t, cfg.AllowTeeDebug)
		require.True(t, cfg.DisableAttestationCheckE2E)
		require.True(t, cfg.AllowPrivateNetworks)
	})
}
func TestParseOptionalBool(t *testing.T) {
	t.Run("empty value defaults to false", func(t *testing.T) {
		res, err := parseOptionalBool("KEY", "")
		require.NoError(t, err)
		require.False(t, res)
	})
	t.Run("invalid value errors", func(t *testing.T) {
		res, err := parseOptionalBool("KEY", "fals")
		require.ErrorContains(t, err, `KEY has invalid bool value "fals"`)
		require.False(t, res)
	})
	t.Run("valid values parse", func(t *testing.T) {
		res, err := parseOptionalBool("KEY", "true")
		require.NoError(t, err)
		require.True(t, res)
		res, err = parseOptionalBool("KEY", "false")
		require.NoError(t, err)
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
