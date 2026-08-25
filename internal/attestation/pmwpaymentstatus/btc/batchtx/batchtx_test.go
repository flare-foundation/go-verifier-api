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

// FuzzSatsFromBTC: arbitrary node-supplied amount strings must never panic, and a
// successful parse must be an in-range satoshi value. The length cap keeps runtime
// bounded regardless of input size.
func FuzzSatsFromBTC(f *testing.F) {
	for _, s := range []string{"", "0", "0.00010000", "21000000", "21000001", "-1", "1e9", "0.000000001", "abc", ".", "0x10", strings.Repeat("9", 100)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		sats, err := SatsFromBTC(s) // must not panic
		if err == nil && (sats < 0 || sats > MaxMoneySat) {
			t.Fatalf("SatsFromBTC(%q) = %d with no error, out of range", s, sats)
		}
	})
}
