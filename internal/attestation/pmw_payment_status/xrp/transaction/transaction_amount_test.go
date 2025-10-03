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

const balanceStr = "balance"

func TestGetReceivedAmount(t *testing.T) {
	t.Run("amount received 1", func(t *testing.T) {
		expectedAmount := big.NewInt(10000000)
		expectedReceiver := "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
		val, err := GetReceivedAmount(&testhelper.TransactionMeta0)
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
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.FinalFields["Balance"] = balanceStr
		modTx := testhelper.TransactionMeta0
		modTx.AffectedNodes = make([]types.AffectedNode, len(testhelper.TransactionMeta0.AffectedNodes))
		copy(modTx.AffectedNodes, testhelper.TransactionMeta0.AffectedNodes)
		modTx.AffectedNodes[0].ModifiedNode = modNode
		val, err := GetReceivedAmount(&modTx)
		require.ErrorContains(t, err, "invalid final balance format")
		require.Nil(t, val)
	})
	t.Run("expect error 3", func(t *testing.T) {
		crNode := testhelper.CopyCreatedNode(testhelper.BasicCreatedNode_tr0)
		crNode.NewFields["Balance"] = balanceStr
		modTx := testhelper.TransactionMeta0
		modTx.AffectedNodes = make([]types.AffectedNode, len(testhelper.TransactionMeta0.AffectedNodes))
		copy(modTx.AffectedNodes, testhelper.TransactionMeta0.AffectedNodes)
		modTx.AffectedNodes[0].CreatedNode = crNode
		val, err := GetReceivedAmount(&modTx)
		require.ErrorContains(t, err, "invalid balance format in CreatedNode")
		require.Nil(t, val)
	})
}

func TestExtractModifiedNode(t *testing.T) {
	t.Run("no 'AccountRoot'", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.LedgerEntryType = "DirectoryNode"
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("FinalFields is nil", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.FinalFields = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("PreviousFields is nil", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.PreviousFields = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("prevBalStr is not string", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.PreviousFields["Balance"] = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("account is not string", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.FinalFields["Account"] = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("finalBalStr is not string", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.FinalFields["Balance"] = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("finalBalStr is not string number", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.FinalFields["Balance"] = balanceStr
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid final balance format")
	})
	t.Run("prevBalStr is not string number", func(t *testing.T) {
		modNode := copyModifiedNode(testhelper.BasicModifiedNode_tr0)
		modNode.PreviousFields["Balance"] = balanceStr
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid previous balance format")
	})
}

func TestExtractCreatedNode(t *testing.T) {
	t.Run("no 'AccountRoot'", func(t *testing.T) {
		modNode := testhelper.CopyCreatedNode(testhelper.BasicCreatedNode_tr0)
		modNode.LedgerEntryType = "Root"
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("NewFields is nil", func(t *testing.T) {
		modNode := testhelper.CopyCreatedNode(testhelper.BasicCreatedNode_tr0)
		modNode.NewFields = nil
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("balanceStr is not string", func(t *testing.T) {
		modNode := testhelper.CopyCreatedNode(testhelper.BasicCreatedNode_tr0)
		modNode.NewFields["Balance"] = nil
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("account is not string", func(t *testing.T) {
		modNode := testhelper.CopyCreatedNode(testhelper.BasicCreatedNode_tr0)
		modNode.NewFields["Account"] = nil
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("balanceStr is not string number", func(t *testing.T) {
		modNode := testhelper.CopyCreatedNode(testhelper.BasicCreatedNode_tr0)
		modNode.NewFields["Balance"] = balanceStr
		val, err := extractFromCreatedNode(modNode)
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
	t.Run("some amount", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(&testhelper.TransactionMeta0, receiver)
		require.NotNil(t, val)
		require.NoError(t, err)
	})
}

func copyModifiedNode(orig *types.ModifiedNode) *types.ModifiedNode {
	if orig == nil {
		return nil
	}
	finalFields := make(map[string]interface{}, len(orig.FinalFields))
	for k, v := range orig.FinalFields {
		finalFields[k] = v
	}
	prevFields := make(map[string]interface{}, len(orig.PreviousFields))
	for k, v := range orig.PreviousFields {
		prevFields[k] = v
	}
	return &types.ModifiedNode{
		LedgerEntryType:   orig.LedgerEntryType,
		LedgerIndex:       orig.LedgerIndex,
		PreviousTxnLgrSeq: orig.PreviousTxnLgrSeq,
		FinalFields:       finalFields,
		PreviousFields:    prevFields,
		PreviousTxnID:     orig.PreviousTxnID,
	}
}
