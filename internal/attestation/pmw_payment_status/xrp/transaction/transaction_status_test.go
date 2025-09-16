package transaction

import (
	"testing"

	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionStatus(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedValue types.TransactionStatus
		expectError   bool
	}{
		{name: "success status", input: "tesSUCCESS", expectedValue: types.Success, expectError: false},
		{name: "receiver fault", input: "tecDST_TAG_NEEDED", expectedValue: types.ReceiverFault, expectError: false},
		{name: "sender fault", input: "tecUNFUNDED", expectedValue: types.SenderFault, expectError: false},
		{name: "invalid input", input: "invalid", expectedValue: 0, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := GetTransactionStatus(tt.input)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedValue, val)
		})
	}
}
