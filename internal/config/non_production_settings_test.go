package config

import (
	"os"
	"testing"

	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/stretchr/testify/require"
)

// unsetNonProductionSettings clears the three variables for the test, restoring the
// caller's values afterwards, so the suite is hermetic whatever environment
// it runs under.
func unsetNonProductionSettings(t *testing.T) {
	t.Helper()
	for _, name := range nonProductionSettingNames {
		t.Setenv(name, "") // registers restoration of the caller's value
		require.NoError(t, os.Unsetenv(name))
	}
}

// On a production chain any assignment of a test-only variable fails the
// boot: true, false, and even an empty assignment all mean the setting
// reached production configuration.
func TestNonProductionSettingsAreRefusedOnProductionChains(t *testing.T) {
	for _, chainID := range []uint64{14, 19} {
		for _, name := range nonProductionSettingNames {
			for _, value := range []string{"true", "false", ""} {
				t.Run(name, func(t *testing.T) {
					unsetNonProductionSettings(t)
					t.Setenv(name, value)
					err := validateNonProductionSettings(chainID, nil)
					require.Error(t, err)
					require.Contains(t, err.Error(), name)
					if value != "" {
						require.NotContains(t, err.Error(), value, "values never reach the log")
					}
				})
			}
		}
	}
}

// A configuration value counts as set even when the process variable is
// absent: the guard holds the configuration being built, not only the
// environment it was read from.
func TestAConfigValueCountsWithoutTheProcessVariable(t *testing.T) {
	unsetNonProductionSettings(t)
	err := validateNonProductionSettings(14, map[string]string{EnvAllowTeeDebug: "true"})
	require.Error(t, err)
	require.Contains(t, err.Error(), EnvAllowTeeDebug)
}

// The builder enforces it end to end: a production configuration carrying a
// test-only value fails even when the process variable is absent.
func TestTheBuilderRefusesAConfigValueOnProduction(t *testing.T) {
	unsetNonProductionSettings(t)
	ClearTeeAvailabilityCheckConfigForTest()
	t.Cleanup(ClearTeeAvailabilityCheckConfigForTest)
	cfg, err := BuildTeeAvailabilityCheckConfig(EnvConfig{
		DestinationChainURLSlug: "flare",
		SourceID:                SourceTEE,
		AttestationType:         fdc2.AvailabilityCheck,
		RelayContractAddress:    "0x0000000000000000000000000000000000000001",
		FlareRPCURL:             "https://rpc.example.com",
		ChainID:                 "14",
		AllowTeeDebug:           "true",
	})
	require.Nil(t, cfg)
	require.ErrorContains(t, err, EnvAllowTeeDebug)
}

// The test networks and the named local chains accept the settings.
func TestNonProductionSettingsAreAllowedOnTestChains(t *testing.T) {
	unsetNonProductionSettings(t)
	for _, chainID := range []uint64{16, 114, 1337, 31337} {
		for _, name := range nonProductionSettingNames {
			t.Setenv(name, "true")
			require.NoError(t, validateNonProductionSettings(chainID, nil))
		}
	}
}

// An unknown chain id is production: a new network is production until it is
// listed, never the other way around.
func TestAnUnknownChainIDIsProduction(t *testing.T) {
	unsetNonProductionSettings(t)
	t.Setenv(EnvAllowTeeDebug, "false")
	require.Error(t, validateNonProductionSettings(99999, nil))
}

// Several offending variables are reported together, in one deterministic
// order, so one boot failure names the whole cleanup.
func TestEveryOffendingVariableIsNamedInOneError(t *testing.T) {
	unsetNonProductionSettings(t)
	for _, name := range nonProductionSettingNames {
		t.Setenv(name, "true")
	}
	err := validateNonProductionSettings(14, nil)
	require.Error(t, err)
	want := EnvAllowTeeDebug + ", " + EnvDisableAttestationCheckE2E + ", " + EnvAllowPrivateNetworks
	require.Contains(t, err.Error(), want)
}

// A clean production environment and configuration pass: absence is the
// production state.
func TestUnsetVariablesPassOnProduction(t *testing.T) {
	unsetNonProductionSettings(t)
	require.NoError(t, validateNonProductionSettings(14, nil))
}
