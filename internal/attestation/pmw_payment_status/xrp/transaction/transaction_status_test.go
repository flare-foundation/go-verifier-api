package transaction

import (
	"testing"

	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionStatus(t *testing.T) {
	tests := []testhelper.TestCase[string, types.TransactionStatus]{
		{Name: "success status", Input: "tesSUCCESS", ExpectedValue: types.Success, ExpectError: false},
		{Name: "receiver fault", Input: "tecDST_TAG_NEEDED", ExpectedValue: types.ReceiverFault, ExpectError: false},
		{Name: "sender fault", Input: "tecUNFUNDED", ExpectedValue: types.SenderFault, ExpectError: false},
		{Name: "invalid input", Input: "invalid", ExpectedValue: 0, ExpectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			val, err := GetTransactionStatus(tc.Input)
			if tc.ExpectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.ExpectedValue, val)
		})
	}
}
