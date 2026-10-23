package config

import (
	"fmt"
	"os"
	"strings"
)

// nonProductionChainIDs are the chain ids of Flare's test networks and the explicitly
// named local development chains. Every other id — Flare (14), Songbird (19)
// and anything unknown — is production: a new network is production until it
// is listed here, never the other way around.
var nonProductionChainIDs = map[uint64]bool{
	16:    true, // Coston
	114:   true, // Coston2
	1337:  true, // local geth --dev
	31337: true, // local Hardhat / Anvil
}

// nonProductionSettingNames are the settings whose runtime effect is acceptable only
// against a test network: each one weakens TEE attestation verification
// (debug TEEs admitted, validation skipped, private TEE URLs allowed). On a
// production chain a deployment must not set them at all — an explicit
// "false", or even an empty assignment, still means the variable reached
// production configuration and the deployment is one edit away from the
// weakened mode.
var nonProductionSettingNames = []string{
	EnvAllowTeeDebug,
	EnvDisableAttestationCheckE2E,
	EnvAllowPrivateNetworks,
}

// validateNonProductionSettings fails the boot of a production-chain
// deployment that sets any test-only variable. Presence is what is checked:
// the configuration value being built AND os.LookupEnv (after any .env file
// was loaded — the latter catches an explicitly empty assignment). The error
// names every offending variable in a fixed order and reports no values.
func validateNonProductionSettings(chainID uint64, values map[string]string) error {
	if nonProductionChainIDs[chainID] {
		return nil
	}
	var set []string
	for _, name := range nonProductionSettingNames {
		_, inEnv := os.LookupEnv(name)
		if inEnv || values[name] != "" {
			set = append(set, name)
		}
	}
	if len(set) == 0 {
		return nil
	}
	return fmt.Errorf("test-only settings set on production chain id %d: %s — unset them (they are allowed only on test networks: Coston 16, Coston2 114, and local chains)",
		chainID, strings.Join(set, ", "))
}
