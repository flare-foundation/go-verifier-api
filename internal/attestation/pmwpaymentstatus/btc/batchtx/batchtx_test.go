package batchtx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSatsFromBTCRejectsOverlongInput: a hostile node could send a valid-looking
// number with millions of digits; it must be rejected on length before big.Rat
// parses it, not decoded.
func TestSatsFromBTCRejectsOverlongInput(t *testing.T) {
	long := "0." + strings.Repeat("0", 1_000_000) + "1"
	_, err := SatsFromBTC(long)
	require.Error(t, err)
	require.ErrorContains(t, err, "too long")
}

// TestSatsFromBTCRejectsAboveMoneySupply: no single amount can exceed 21M BTC.
func TestSatsFromBTCRejectsAboveMoneySupply(t *testing.T) {
	_, err := SatsFromBTC("21000001")
	require.Error(t, err)
	require.ErrorContains(t, err, "money supply")
}

// TestSatsFromBTCAcceptsValidAmount pins the exact conversion.
func TestSatsFromBTCAcceptsValidAmount(t *testing.T) {
	sats, err := SatsFromBTC("0.00010000")
	require.NoError(t, err)
	require.Equal(t, int64(10000), sats)
}
