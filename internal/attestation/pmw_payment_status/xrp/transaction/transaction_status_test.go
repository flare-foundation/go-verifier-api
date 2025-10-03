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
		errorMessage  string
	}{
		{name: "success status", input: "tesSUCCESS", expectedValue: types.Success, expectError: false},
		{name: "receiver fault", input: "tecDST_TAG_NEEDED", expectedValue: types.ReceiverFault, expectError: false},
		{name: "sender fault", input: "tecUNFUNDED", expectedValue: types.SenderFault, expectError: false},
		{name: "sender fault 2", input: "tefALREADY", expectedValue: types.SenderFault, expectError: false},
		{name: "invalid input", input: "invalid", expectedValue: 0, expectError: true, errorMessage: "unexpected transaction status prefix"},
		{name: "too short input", input: "te", expectedValue: 0, expectError: true, errorMessage: "transaction result too short"},
		{name: "unknown tec code", input: "tecINVALID_NOT_KNOWN_CODE", expectedValue: 0, expectError: true, errorMessage: "unknown tec error code"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := GetTransactionStatus(tt.input)
			if tt.expectError {
				require.ErrorContains(t, err, tt.errorMessage)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedValue, val)
		})
	}
}
