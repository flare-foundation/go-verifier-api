package transaction

import (
	"testing"

	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
)

func TestGetTransactionStatus(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedStatus types.TransactionStatus
		expectError    bool
	}{
		{"success status", "tesSUCCESS", types.Success, false},
		{"receiver fault", "tecDST_TAG_NEEDED", types.ReceiverFault, false},
		{"sender fault", "tecUNFUNDED", types.SenderFault, false},
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
