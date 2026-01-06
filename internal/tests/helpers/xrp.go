package helpers

import (
	"github.com/flare-foundation/go-flare-common/pkg/xrpl/transactions"
	"github.com/flare-foundation/go-verifier-api/internal/attestation/pmw_payment_status/xrp/types"
)

const sequenceTx0 = 10110067

var PaymentTransaction0 = types.RawTransactionData{
	CommonFields: transactions.CommonFields{
		Account:         "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
		Fee:             "100",
		TransactionType: "Payment",
		Sequence:        sequenceTx0,
	},
	MetaData: types.TransactionMetaData{
		TransactionResult: "tesSUCCESS",
		AffectedNodes: []types.AffectedNode{
			{
				CreatedNode: &types.CreatedNode{
					LedgerEntryType: "AccountRoot",
					LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
					NewFields: map[string]interface{}{
						"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
						"Balance": "10000000",
					},
				},
			}, {
				ModifiedNode: &types.ModifiedNode{
					LedgerEntryType: "AccountRoot",
					LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
					FinalFields: map[string]interface{}{
						"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
						"Balance": "1922391830527342",
					},
					PreviousFields: map[string]interface{}{
						"Balance": "1922391840527354",
					},
				},
			},
		},
	},
}
var TransactionMeta0_error0 = types.TransactionMetaData{
	TransactionResult: "tesSUCCESS",
	AffectedNodes: []types.AffectedNode{
		{
			CreatedNode: &types.CreatedNode{
				LedgerEntryType: "AccountRoot",
				LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
				NewFields: map[string]interface{}{
					"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
					"Balance": "10000000",
				},
			},
		}, {
			ModifiedNode: &types.ModifiedNode{
				LedgerEntryType: "AccountRoot",
				LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
				FinalFields: map[string]interface{}{
					"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
					"Balance": "balance",
				},
				PreviousFields: map[string]interface{}{
					"Balance": "1922391840527354",
				},
			},
		},
	},
}
var PaymentTransaction0_error0 = types.RawTransactionData{
	CommonFields: transactions.CommonFields{
		Account:         "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
		Fee:             "100",
		TransactionType: "Payment",
		Sequence:        sequenceTx0,
	},
	MetaData: types.TransactionMetaData{
		TransactionResult: "tesSUCCESS",
		AffectedNodes: []types.AffectedNode{
			{
				CreatedNode: &types.CreatedNode{
					LedgerEntryType: "AccountRoot",
					LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
					NewFields: map[string]interface{}{
						"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
						"Balance": "balance",
					},
				},
			}, {
				ModifiedNode: &types.ModifiedNode{
					LedgerEntryType: "AccountRoot",
					LedgerIndex:     "31CCE9D28412FF973E9AB6D0FA219BACF19687D9A2456A0C2ABC3280E9D47E37",
					FinalFields: map[string]interface{}{
						"Account": "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe",
						"Balance": "balance",
					},
					PreviousFields: map[string]interface{}{
						"Balance": "1922391840527354",
					},
				},
			},
		},
	},
}

var TransactionMeta1 = types.TransactionMetaData{
	TransactionResult: "tesSUCCESS",
	AffectedNodes: []types.AffectedNode{
		{
			ModifiedNode: &types.ModifiedNode{
				LedgerEntryType: "AccountRoot",
				LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
				FinalFields: map[string]interface{}{
					"Account": "rp2X3jj55rZySZFgJz1q4xuFjAb2JZXyWK",
					"Balance": "9989876",
				},
				PreviousFields: map[string]interface{}{
					"Balance": "9999976",
				},
			},
		},
		{
			ModifiedNode: &types.ModifiedNode{
				LedgerEntryType: "AccountRoot",
				LedgerIndex:     "367AEF9941B4693008A3D0680776743E94527F4066FABAAA0C62FBC91F5E56B9",
				FinalFields: map[string]interface{}{
					"Account": "rN5N6fJbc8xyViPDeQFMQMpYfVHuxSGV2G",
					"Balance": "190310000",
				},
				PreviousFields: map[string]interface{}{
					"Balance": "190300000",
				},
			},
		},
	},
}

const sequenceAccountRootTx = 11834748

var AccountRootTx = types.RawTransactionData{
	CommonFields: transactions.CommonFields{
		Account:         "rw93HsrEYDtcMxEu4RQhFvH9pXH1TnCZu4",
		Fee:             "12",
		TransactionType: "AccountSet",
		Sequence:        sequenceAccountRootTx,
	},
	MetaData: types.TransactionMetaData{
		TransactionResult: "tesSUCCESS",
		AffectedNodes: []types.AffectedNode{
			{
				ModifiedNode: &types.ModifiedNode{
					FinalFields: map[string]interface{}{
						"Account":    "rw93HsrEYDtcMxEu4RQhFvH9pXH1TnCZu4",
						"Balance":    "99999988",
						"Flags":      0,
						"OwnerCount": 0,
						"Sequence":   sequenceAccountRootTx + 1,
					},
					LedgerEntryType: "AccountRoot",
					LedgerIndex:     "E7F60ED41F22C2C0F00E7BA7E1E32267383664AA870FA34303C12070DA42C16F",
					PreviousFields: map[string]interface{}{
						"Balance":  "100000000",
						"Sequence": sequenceAccountRootTx,
					},
					PreviousTxnID:     "AC84EBC5A6222EAB30CFC670A738DE65F9DF25AF53536556388C8845B83CEEB3",
					PreviousTxnLgrSeq: sequenceAccountRootTx,
				},
			},
		},
	},
}
