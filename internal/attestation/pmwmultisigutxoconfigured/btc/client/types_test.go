package client

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBtcToSat(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"0.00010000", 10_000, false},
		{"0.00009999", 9_999, false},
		{"1", 100_000_000, false},
		{"1.5", 150_000_000, false},
		{"21000000", 2_100_000_000_000_000, false}, // total BTC supply, no float rounding
		{"0", 0, false},
		{"0.000000001", 0, true}, // 9 fractional digits — too precise
		{"-1", 0, true},
		{"", 0, true},
		{"1.2.3", 0, true},
		{"abc", 0, true},
		{"21000001", 0, true},                   // just above the money supply
		{"99999999999999999999999999", 0, true}, // oversized: would overflow int64
		{"92233720368.54775808", 0, true},       // ~int64 max in sat: impossible, rejected
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := btcToSat(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestGetTxOutValueSat(t *testing.T) {
	o := &GetTxOut{Value: json.Number("0.00010000")}
	sat, err := o.ValueSat()
	require.NoError(t, err)
	require.Equal(t, int64(10_000), sat)
}

// TestGetTxOutResponseNullResult confirms a null result unmarshals to a nil
// pointer (the not-found / spent case), not a zero-value struct.
func TestGetTxOutResponseNullResult(t *testing.T) {
	var resp getTxOutResponse
	require.NoError(t, json.Unmarshal([]byte(`{"result":null,"error":null,"id":"x"}`), &resp))
	require.Nil(t, resp.Result)
	require.Nil(t, resp.Error)
}
