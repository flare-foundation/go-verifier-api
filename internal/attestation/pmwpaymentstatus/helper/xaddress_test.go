package helper

import (
	"testing"

	addresscodec "github.com/Peersyst/xrpl-go/address-codec"
	"github.com/stretchr/testify/require"
)

const testClassicAddress = "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"

func testXAddress(t *testing.T) string {
	t.Helper()
	xAddr, err := addresscodec.ClassicAddressToXAddress(testClassicAddress, 0, false, false)
	require.NoError(t, err)
	return xAddr
}

func testXAddressWithTag(t *testing.T) string {
	t.Helper()
	xAddr, err := addresscodec.ClassicAddressToXAddress(testClassicAddress, 12345, true, false)
	require.NoError(t, err)
	return xAddr
}

func TestNormalizeClassicAddress(t *testing.T) {
	addr, err := NormalizeAddress(testClassicAddress)
	require.NoError(t, err)
	require.Equal(t, testClassicAddress, addr)
}

func TestNormalizeXAddress(t *testing.T) {
	xAddr := testXAddress(t)
	addr, err := NormalizeAddress(xAddr)
	require.NoError(t, err)
	require.Equal(t, testClassicAddress, addr)
}

func TestNormalizeXAddressWithTag(t *testing.T) {
	xAddr := testXAddressWithTag(t)
	addr, err := NormalizeAddress(xAddr)
	require.NoError(t, err)
	require.Equal(t, testClassicAddress, addr)
}

func TestNormalizeEmptyString(t *testing.T) {
	addr, err := NormalizeAddress("")
	require.NoError(t, err)
	require.Equal(t, "", addr)
}

func TestNormalizeInvalidString(t *testing.T) {
	addr, err := NormalizeAddress("not-an-address")
	require.NoError(t, err)
	require.Equal(t, "not-an-address", addr) // passthrough, not an X-address
}
