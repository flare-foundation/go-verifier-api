package helper

import (
	addresscodec "github.com/Peersyst/xrpl-go/address-codec"
)

// NormalizeAddress converts an X-address to a classic r... address.
// If the input is already a classic address, it is returned unchanged.
func NormalizeAddress(addr string) (string, error) {
	if !addresscodec.IsValidXAddress(addr) {
		return addr, nil
	}
	classic, _, _, err := addresscodec.XAddressToClassicAddress(addr)
	return classic, err
}
