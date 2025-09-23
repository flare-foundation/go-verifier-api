package testhelper

import types "github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/type"

func CopyCreatedNode(orig *types.CreatedNode) *types.CreatedNode {
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

var TransactionMeta0 = types.TransactionMetaData{
	TransactionResult: "tesSUCCESS",
	AffectedNodes: []types.AffectedNode{
		{
			CreatedNode: BasicCreatedNode_tr0,
		}, {
			ModifiedNode: BasicModifiedNode_tr0,
		},
	},
}
var BasicCreatedNode_tr0 = &types.CreatedNode{
	LedgerEntryType: "AccountRoot",
	LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
	NewFields: map[string]interface{}{
		"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
		"Balance": "10000000",
	},
}
var BasicModifiedNode_tr0 = &types.ModifiedNode{
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
var BasicModifiedNode_tr1_0 = &types.ModifiedNode{
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
var BasicModifiedNode_tr1_1 = &types.ModifiedNode{
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
var TransactionMeta1 = types.TransactionMetaData{
	TransactionResult: "tesSUCCESS",
	AffectedNodes: []types.AffectedNode{
		{
			ModifiedNode: BasicModifiedNode_tr1_0,
		},
		{
			ModifiedNode: BasicModifiedNode_tr1_1,
		},
	},
}
