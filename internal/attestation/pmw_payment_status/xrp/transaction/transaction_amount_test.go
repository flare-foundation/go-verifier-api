package transaction

import (
	"math/big"
	"testing"

	types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"
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
	t.Run("Amount received 1", func(t *testing.T) {
		expectedAmount := big.NewInt(10000000)
		expectedReceiver := "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
		val, err := GetReceivedAmount(&transaction0)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.Equal(t, val[0].Amount, expectedAmount)
		require.Equal(t, val[0].Address, expectedReceiver)
	})
	t.Run("Amount received 2", func(t *testing.T) {
		expectedAmount := big.NewInt(10000)
		expectedReceiver := "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G"
		val, err := GetReceivedAmount(&transaction1)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.Equal(t, val[0].Amount, expectedAmount)
		require.Equal(t, val[0].Address, expectedReceiver)
	})
	t.Run("Expect error", func(t *testing.T) {
		val, err := GetReceivedAmount(nil)
		require.ErrorContains(t, err, "transaction meta is not available, thus received amounts cannot be calculated")
		require.Nil(t, val)
	})
	t.Run("Expect error 2", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.FinalFields["Balance"] = "balance"
		modTx := transaction0
		modTx.AffectedNodes = make([]types.AffectedNode, len(transaction0.AffectedNodes))
		copy(modTx.AffectedNodes, transaction0.AffectedNodes)
		modTx.AffectedNodes[0].ModifiedNode = modNode
		val, err := GetReceivedAmount(&modTx)
		require.ErrorContains(t, err, "invalid final balance format")
		require.Nil(t, val)
	})
	t.Run("Expect error 3", func(t *testing.T) {
		crNode := copyCreatedNode(basicCreatedNode_tr0)
		crNode.NewFields["Balance"] = "balance"
		modTx := transaction0
		modTx.AffectedNodes = make([]types.AffectedNode, len(transaction0.AffectedNodes))
		copy(modTx.AffectedNodes, transaction0.AffectedNodes)
		modTx.AffectedNodes[0].CreatedNode = crNode
		val, err := GetReceivedAmount(&modTx)
		require.ErrorContains(t, err, "invalid balance format in CreatedNode")
		require.Nil(t, val)
	})
}

func TestExtractModifiedNode(t *testing.T) {
	t.Run("No 'AccountRoot'", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.LedgerEntryType = "DirectoryNode"
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("FinalFields is nil", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.FinalFields = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("PreviousFields is nil", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.PreviousFields = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("prevBalStr is not string", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.PreviousFields["Balance"] = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("account is not string", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.FinalFields["Account"] = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("finalBalStr is not string", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.FinalFields["Balance"] = nil
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("finalBalStr is not string number", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.FinalFields["Balance"] = "balance"
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid final balance format")
	})
	t.Run("prevBalStr is not string number", func(t *testing.T) {
		modNode := copyModifiedNode(basicModifiedNode_tr0)
		modNode.PreviousFields["Balance"] = "balance"
		val, err := extractFromModifiedNode(modNode)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid previous balance format")
	})
}

func TestExtractCreatedNode(t *testing.T) {
	t.Run("No 'AccountRoot'", func(t *testing.T) {
		modNode := copyCreatedNode(basicCreatedNode_tr0)
		modNode.LedgerEntryType = "Root"
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("NewFields is nil", func(t *testing.T) {
		modNode := copyCreatedNode(basicCreatedNode_tr0)
		modNode.NewFields = nil
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("balanceStr is not string", func(t *testing.T) {
		modNode := copyCreatedNode(basicCreatedNode_tr0)
		modNode.NewFields["Balance"] = nil
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("account is not string", func(t *testing.T) {
		modNode := copyCreatedNode(basicCreatedNode_tr0)
		modNode.NewFields["Account"] = nil
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.Nil(t, err)
	})
	t.Run("balanceStr is not string number", func(t *testing.T) {
		modNode := copyCreatedNode(basicCreatedNode_tr0)
		modNode.NewFields["Balance"] = "balance"
		val, err := extractFromCreatedNode(modNode)
		require.Nil(t, val)
		require.ErrorContains(t, err, "invalid balance format in CreatedNode")
	})
}

func TestFindReceivedAmountForAddress(t *testing.T) {
	receiver := "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK"
	t.Run("Error", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(nil, receiver)
		require.Nil(t, val)
		require.ErrorContains(t, err, "transaction meta is not available, thus received amounts cannot be calculated")
	})
	t.Run("No amount", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(&transaction1, receiver)
		require.Equal(t, val, big.NewInt(0))
		require.NoError(t, err)
	})
	t.Run("Some amount", func(t *testing.T) {
		val, err := FindReceivedAmountForAddress(&transaction0, receiver)
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

func copyCreatedNode(orig *types.CreatedNode) *types.CreatedNode {
	if orig == nil {
		return nil
	}
	newFields := make(map[string]interface{}, len(orig.NewFields))
	for k, v := range orig.NewFields {
		newFields[k] = v
	}
	return &types.CreatedNode{
		LedgerEntryType: orig.LedgerEntryType,
		LedgerIndex:     orig.LedgerIndex,
		NewFields:       newFields,
	}
}

var transaction0 = types.TransactionMetaData{
	TransactionResult: "tesSUCCESS",
	AffectedNodes: []types.AffectedNode{
		{
			CreatedNode: basicCreatedNode_tr0,
		}, {
			ModifiedNode: basicModifiedNode_tr0,
		},
	},
}
var basicCreatedNode_tr0 = &types.CreatedNode{
	LedgerEntryType: "AccountRoot",
	LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
	NewFields: map[string]interface{}{
		"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
		"Balance": "10000000",
	},
}
var basicModifiedNode_tr0 = &types.ModifiedNode{
	LedgerEntryType: "AccountRoot",
	LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
	FinalFields: map[string]interface{}{
		"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
		"Balance": "1922391830527342",
	},
	PreviousFields: map[string]interface{}{
		"Balance": "1922391840527354",
	},
}
var basicModifiedNode_tr1_0 = &types.ModifiedNode{
	LedgerEntryType: "AccountRoot",
	LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
	FinalFields: map[string]interface{}{
		"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
		"Balance": "9989876",
	},
	PreviousFields: map[string]interface{}{
		"Balance": "9999976",
	},
}
var basicModifiedNode_tr1_1 = &types.ModifiedNode{
	LedgerEntryType: "AccountRoot",
	LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
	FinalFields: map[string]interface{}{
		"Account": "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G",
		"Balance": "190310000",
	},
	PreviousFields: map[string]interface{}{
		"Balance": "190300000",
	},
}
var transaction1 = types.TransactionMetaData{
	TransactionResult: "tesSUCCESS",
	AffectedNodes: []types.AffectedNode{
		{
			ModifiedNode: basicModifiedNode_tr1_0,
		},
		{
			ModifiedNode: basicModifiedNode_tr1_1,
		},
	},
}
