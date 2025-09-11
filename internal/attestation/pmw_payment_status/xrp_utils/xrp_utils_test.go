package xrputils_test

import (
	testutil "github.com/flare-foundation/go-verifier-api/internal/test_util"
	"testing"

	xrptypes "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/types"
	xrputils "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp_utils"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionStatus(t *testing.T) {
	tests := []testutil.TestCase[string, xrptypes.TransactionStatus]{
		{Name: "success status", Input: "tesSUCCESS", ExpectedValue: xrptypes.Success, ExpectError: false},
		{Name: "receiver fault", Input: "tecDST_TAG_NEEDED", ExpectedValue: xrptypes.ReceiverFault, ExpectError: false},
		{Name: "sender fault", Input: "tecUNFUNDED", ExpectedValue: xrptypes.SenderFault, ExpectError: false},
		{Name: "invalid input", Input: "invalid", ExpectedValue: 0, ExpectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			val, err := xrputils.GetTransactionStatus(tc.Input)
			if tc.ExpectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.ExpectedValue, val)
		})
	}
}
