package transaction

import (
	"math/big"
	"testing"

	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
	testhelper "github.com/flare-foundation/go-verifier-api/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestGetStringField(t *testing.T) {
	m := map[string]interface{}{
		"key1": "val1",
		"key2": 1234,
	}
	t.Run("valid string field", func(t *testing.T) {
		val, ok := getStringField(m, "key1")
		require.True(t, ok)
		require.Equal(t, "val1", val)
	})
	t.Run("number field", func(t *testing.T) {
		_, ok := getStringField(m, "key2")
		require.False(t, ok)
	})
	t.Run("missing field", func(t *testing.T) {
		_, ok := getStringField(m, "missing")
		require.False(t, ok)
	})
}

func TestGetReceivedAmount(t *testing.T) {
	t.Run("amount received 1", func(t *testing.T) {
		expectedAmount := big.NewInt(10000000)
		expectedReceiver := "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
		val, err := GetReceivedAmount(&testhelper.PaymentTransaction0.MetaData)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.Equal(t, expectedAmount, val[0].Amount)
		require.Equal(t, expectedReceiver, val[0].Address)
	})
	t.Run("amount received 2", func(t *testing.T) {
		expectedAmount := big.NewInt(10000)
		expectedReceiver := "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G"
		val, err := GetReceivedAmount(&testhelper.TransactionMeta1)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.Equal(t, expectedAmount, val[0].Amount)
		require.Equal(t, expectedReceiver, val[0].Address)
	})
	t.Run("expect error", func(t *testing.T) {
		val, err := GetReceivedAmount(nil)
		require.ErrorContains(t, err, "transaction meta is not available, thus received amounts cannot be calculated")
		require.Nil(t, val)
	})
	t.Run("expect error 2", func(t *testing.T) {
		val, err := GetReceivedAmount(&testhelper.TransactionMeta0_error0)
		require.ErrorContains(t, err, "invalid final balance format")
		require.Nil(t, val)
	})
	t.Run("expect error 3", func(t *testing.T) {
		val, err := GetReceivedAmount(&testhelper.PaymentTransaction0_error0.MetaData)
		require.ErrorContains(t, err, "invalid balance format in CreatedNode")
		require.Nil(t, val)
	})
}

func TestExtractModifiedNode(t *testing.T) {
	t.Run("no 'AccountRoot'", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "DirectoryNode",
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("FinalFields is nil", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("PreviousFields is nil", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
			FinalFields: map[string]interface{}{
				"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
				"Balance": "1922391830527342",
			},
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("prevBalStr is not string", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
			FinalFields: map[string]interface{}{
				"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
				"Balance": "1922391830527342",
			},
			PreviousFields: map[string]interface{}{},
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("account is not string", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
			FinalFields: map[string]interface{}{
				"Balance": "1922391830527342",
			},
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("finalBalStr is not string", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
			FinalFields: map[string]interface{}{
				"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
			},
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("finalBalStr is not string number", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
			FinalFields: map[string]interface{}{
				"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
				"Balance": "balance",
			},
			PreviousFields: map[string]interface{}{
				"Balance": "1922391840527354",
			},
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid final balance format")
	})
	t.Run("prevBalStr is not string number", func(t *testing.T) {
		var node = &types.ModifiedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
			FinalFields: map[string]interface{}{
				"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
				"Balance": "1922391830527342",
			},
			PreviousFields: map[string]interface{}{
				"Balance": "balance",
			},
		}
		val, err := extractFromModifiedNode(node)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid previous balance format")
	})
}

func TestExtractCreatedNode(t *testing.T) {
	t.Run("no 'AccountRoot'", func(t *testing.T) {
		node := &types.CreatedNode{
			LedgerEntryType: "Root",
		}
		val, err := extractFromCreatedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("NewFields is nil", func(t *testing.T) {
		node := &types.CreatedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
		}
		val, err := extractFromCreatedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("balanceStr is not string", func(t *testing.T) {
		node := &types.CreatedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
			NewFields: map[string]interface{}{
				"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
			},
		}
		val, err := extractFromCreatedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("account is not string", func(t *testing.T) {
		node := &types.CreatedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
			NewFields: map[string]interface{}{
				"Balance": "10000000",
			},
		}
		val, err := extractFromCreatedNode(node)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("balanceStr is not string number", func(t *testing.T) {
		node := &types.CreatedNode{
			LedgerEntryType: "AccountRoot",
			LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
			NewFields: map[string]interface{}{
				"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
				"Balance": "balance",
			},
		}
		val, err := extractFromCreatedNode(node)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid balance format in CreatedNode")
	})
}

func TestFindReceivedAmountForAddress(t *testing.T) {
	receiver := "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
	t.Run("error", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(nil, receiver)
		require.Nil(t, val)
		require.ErrorContains(t, err, "transaction meta is not available, thus received amounts cannot be calculated")
	})
	t.Run("no amount", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(&testhelper.TransactionMeta1, receiver)
		require.Equal(t, big.NewInt(0), val)
		require.NoError(t, err)
	})
	t.Run("no amount 2", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(&testhelper.AccountRootTx.MetaData, receiver)
		require.Equal(t, big.NewInt(0), val)
		require.NoError(t, err)
	})
	t.Run("some amount", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(&testhelper.PaymentTransaction0.MetaData, receiver)
		require.NotNil(t, val)
		require.NoError(t, err)
	})
}
