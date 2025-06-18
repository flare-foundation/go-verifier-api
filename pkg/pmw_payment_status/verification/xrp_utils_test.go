package verification

import (
	"testing"

	xrptypes "gitlab.com/urskak/verifier-api/pkg/pmw_payment_status/types"
)

func TestGetTransactionStatus(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedStatus xrptypes.TransactionStatus
		expectError    bool
	}{
		{"success status", "tesSUCCESS", xrptypes.Success, false},
		{"receiver fault", "tecDST_TAG_NEEDED", xrptypes.ReceiverFault, false},
		{"sender fault", "tecUNFUNDED", xrptypes.SenderFault, false},
		{"invalid input", "invalid", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := GetTransactionStatus(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error for input %q, got none", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error for input %q: %v", tc.input, err)
			}
			if val != tc.expectedStatus {
				t.Fatalf("Expected %d, got %d", tc.expectedStatus, val)
			}
		})
	}
}
