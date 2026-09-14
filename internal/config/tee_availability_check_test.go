package config

import (
	"encoding/pem"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/stretchr/testify/require"
)

func TestBuildTeeAvailabilityCheckConfigError(t *testing.T) {
	for _, src := range []SourceName{SourceXRP, SourceTestXRP, ""} {
		t.Run("unsupported SOURCE_ID "+string(src)+" fails the boot", func(t *testing.T) {
			envConfig := EnvConfig{
				SourceID:             src,
				AttestationType:      fdc2.AvailabilityCheck,
				RelayContractAddress: "0x0000000000000000000000000000000000000001",
				FlareRPCURL:          "https://rpc.example.com",
				ChainID:              "16",
			}
			cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
			require.Nil(t, cfg)
			require.ErrorContains(t, err, "unsupported SOURCE_ID")
			require.ErrorContains(t, err, "for TeeAvailabilityCheck")
		})
	}
	t.Run("missing required fields", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:        SourceTEE,
			AttestationType: "UnknownType",
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.Nil(t, cfg)
		require.ErrorContains(t, err, "missing environment variables: RELAY_CONTRACT_ADDRESS, FLARE_RPC_URL")
	})
	t.Run("invalid RELAY_CONTRACT_ADDRESS hex", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:             SourceTEE,
			AttestationType:      "UnknownType",
			RelayContractAddress: "not-hex",
			FlareRPCURL:          "URL",
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
			FlareRPCURL:          "URL",
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
				FlareRPCURL:          "https://rpc.example.com",
			}
			tc.mutate(&envConfig)
			cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
			require.Nil(t, cfg)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
	cutoverCases := []struct {
		name    string
		mutate  func(*EnvConfig)
		wantErr string
	}{
		{
			name:    "next address without first policy id fails the boot",
			mutate:  func(c *EnvConfig) { c.RelayCutoverContractAddress = "0x0000000000000000000000000000000000000002" },
			wantErr: "must be set together",
		},
		{
			name:    "first policy id without next address fails the boot",
			mutate:  func(c *EnvConfig) { c.RelayCutoverStartingRewardEpoch = "250" },
			wantErr: "must be set together",
		},
		{
			name: "invalid next address hex fails the boot",
			mutate: func(c *EnvConfig) {
				c.RelayCutoverContractAddress = "not-hex"
				c.RelayCutoverStartingRewardEpoch = "250"
			},
			wantErr: "RELAY_CUTOVER_CONTRACT_ADDRESS is not a valid hex address",
		},
		{
			name: "next address equal to the current one fails the boot",
			mutate: func(c *EnvConfig) {
				c.RelayCutoverContractAddress = "0x0000000000000000000000000000000000000001"
				c.RelayCutoverStartingRewardEpoch = "250"
			},
			wantErr: "RELAY_CUTOVER_CONTRACT_ADDRESS must differ from RELAY_CONTRACT_ADDRESS",
		},
		{
			name: "non-numeric first policy id fails the boot",
			mutate: func(c *EnvConfig) {
				c.RelayCutoverContractAddress = "0x0000000000000000000000000000000000000002"
				c.RelayCutoverStartingRewardEpoch = "soon"
			},
			wantErr: "RELAY_CUTOVER_STARTING_REWARD_EPOCH must be a base-10 uint24",
		},
		{
			// Reward-epoch ids are uint24 on chain; a wider cutoff would boot but
			// could never be reached, so the switch would silently never happen.
			name: "first policy id beyond uint24 fails the boot",
			mutate: func(c *EnvConfig) {
				c.RelayCutoverContractAddress = "0x0000000000000000000000000000000000000002"
				c.RelayCutoverStartingRewardEpoch = "16777216" // 1<<24, first invalid value
			},
			wantErr: "RELAY_CUTOVER_STARTING_REWARD_EPOCH must be a base-10 uint24",
		},
		{
			name: "zero first policy id fails the boot",
			mutate: func(c *EnvConfig) {
				c.RelayCutoverContractAddress = "0x0000000000000000000000000000000000000002"
				c.RelayCutoverStartingRewardEpoch = "0"
			},
			wantErr: "RELAY_CUTOVER_STARTING_REWARD_EPOCH must be non-zero",
		},
	}
	for _, tc := range cutoverCases {
		t.Run(tc.name, func(t *testing.T) {
			envConfig := EnvConfig{
				SourceID:             SourceTEE,
				AttestationType:      fdc2.AvailabilityCheck,
				RelayContractAddress: "0x0000000000000000000000000000000000000001",
				FlareRPCURL:          "https://rpc.example.com",
				ChainID:              "16",
			}
			tc.mutate(&envConfig)
			cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
			require.Nil(t, cfg)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestBuildTeeAvailabilityCheckConfigSuccess(t *testing.T) {
	const (
		validAudience = "test-audience"
		validChainID  = "16"
	)
	t.Run("defaults", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:             SourceTEE,
			AttestationType:      fdc2.AvailabilityCheck,
			RelayContractAddress: "0x0000000000000000000000000000000000000001",
			FlareRPCURL:          "https://rpc.example.com",
			TeeAudience:          validAudience,
			ChainID:              validChainID,
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.False(t, cfg.AllowTeeDebug)
		require.False(t, cfg.DisableAttestationCheckE2E)
		require.False(t, cfg.AllowPrivateNetworks)
		require.NotEqual(t, cfg.RelayContractAddress, [20]byte{})
		require.Equal(t, "https://rpc.example.com", cfg.FlareRPCURL)
		require.NotNil(t, cfg.GoogleRootCertificate)
		require.Equal(t, validAudience, cfg.TeeAudience)
		require.Equal(t, uint64(16), cfg.ChainID)
		// No cutover configured: zero address and id.
		require.Equal(t, [20]byte{}, [20]byte(cfg.RelayCutoverContractAddress))
		require.Zero(t, cfg.RelayCutoverStartingRewardEpoch)
	})
	t.Run("relay cutover configured", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                        SourceTEE,
			AttestationType:                 fdc2.AvailabilityCheck,
			RelayContractAddress:            "0x0000000000000000000000000000000000000001",
			RelayCutoverContractAddress:     "0x0000000000000000000000000000000000000002",
			RelayCutoverStartingRewardEpoch: "250",
			FlareRPCURL:                     "https://rpc.example.com",
			TeeAudience:                     validAudience,
			ChainID:                         validChainID,
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.Equal(t, "0x0000000000000000000000000000000000000002", cfg.RelayCutoverContractAddress.Hex())
		require.Equal(t, uint32(250), cfg.RelayCutoverStartingRewardEpoch)
	})
	t.Run("allow private networks enabled", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:             SourceTEE,
			AttestationType:      fdc2.AvailabilityCheck,
			RelayContractAddress: "0x0000000000000000000000000000000000000001",
			FlareRPCURL:          "https://rpc.example.com",
			AllowPrivateNetworks: "true",
			TeeAudience:          validAudience,
			ChainID:              validChainID,
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.True(t, cfg.AllowPrivateNetworks)
	})
	t.Run("all flags enabled skips audience requirement but still requires CHAIN_ID", func(t *testing.T) {
		envConfig := EnvConfig{
			SourceID:                   SourceTEE,
			AttestationType:            fdc2.AvailabilityCheck,
			RelayContractAddress:       "0x0000000000000000000000000000000000000001",
			FlareRPCURL:                "https://rpc.example.com",
			AllowTeeDebug:              "true",
			DisableAttestationCheckE2E: "true",
			AllowPrivateNetworks:       "true",
			ChainID:                    validChainID,
		}
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.True(t, cfg.AllowTeeDebug)
		require.True(t, cfg.DisableAttestationCheckE2E)
		require.True(t, cfg.AllowPrivateNetworks)
		require.Equal(t, uint64(16), cfg.ChainID)
	})
}

func TestBuildTeeAvailabilityCheckConfigPolicyFields(t *testing.T) {
	base := EnvConfig{
		SourceID:             SourceTEE,
		AttestationType:      fdc2.AvailabilityCheck,
		RelayContractAddress: "0x0000000000000000000000000000000000000001",
		FlareRPCURL:          "https://rpc.example.com",
		ChainID:              "16",
	}
	t.Run("unset TEE_AUDIENCE defaults to DefaultTeeAudience", func(t *testing.T) {
		envConfig := base
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.Equal(t, DefaultTeeAudience, cfg.TeeAudience)
	})
	t.Run("explicit TEE_AUDIENCE overrides the default", func(t *testing.T) {
		envConfig := base
		envConfig.TeeAudience = "custom-audience"
		cfg, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.NoError(t, err)
		require.Equal(t, "custom-audience", cfg.TeeAudience)
	})
	t.Run("missing CHAIN_ID", func(t *testing.T) {
		envConfig := base
		envConfig.ChainID = ""
		envConfig.TeeAudience = "aud"
		_, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.ErrorContains(t, err, "missing environment variables: CHAIN_ID")
	})
	t.Run("invalid CHAIN_ID", func(t *testing.T) {
		envConfig := base
		envConfig.TeeAudience = "aud"
		envConfig.ChainID = "not-a-number"
		_, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.ErrorContains(t, err, "CHAIN_ID must be a base-10 uint64")
	})
	t.Run("zero CHAIN_ID rejected", func(t *testing.T) {
		envConfig := base
		envConfig.TeeAudience = "aud"
		envConfig.ChainID = "0"
		_, err := BuildTeeAvailabilityCheckConfig(envConfig)
		require.ErrorContains(t, err, "CHAIN_ID must be non-zero")
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
