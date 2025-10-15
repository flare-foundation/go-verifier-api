package types

import (
	"math/big"

	"github.com/flare-foundation/go-flare-common/pkg/xrpl/transactions"
)

type TransactionStatus uint8

const (
	Success TransactionStatus = iota
	Reverted
)

type RawTransactionData struct {
	transactions.CommonFields
	MetaData TransactionMetaData `json:"metaData"`
}

type TransactionMetaData struct {
	TransactionResult string         `json:"TransactionResult"`
	AffectedNodes     []AffectedNode `json:"AffectedNodes"`
}

type AffectedNode struct {
	CreatedNode  *CreatedNode  `json:"CreatedNode,omitempty"`
	DeletedNode  *DeletedNode  `json:"DeletedNode,omitempty"`
	ModifiedNode *ModifiedNode `json:"ModifiedNode,omitempty"`
}

type CreatedNode struct {
	LedgerEntryType string                 `json:"LedgerEntryType"`
	LedgerIndex     string                 `json:"LedgerIndex"`
	NewFields       map[string]interface{} `json:"NewFields"`
}

type DeletedNode struct {
	LedgerEntryType string                 `json:"LedgerEntryType"`
	LedgerIndex     string                 `json:"LedgerIndex"`
	FinalFields     map[string]interface{} `json:"FinalFields"`
	PreviousFields  map[string]interface{} `json:"PreviousFields,omitempty"`
}

type ModifiedNode struct {
	LedgerEntryType   string                 `json:"LedgerEntryType"`
	LedgerIndex       string                 `json:"LedgerIndex"`
	FinalFields       map[string]interface{} `json:"FinalFields"`
	PreviousFields    map[string]interface{} `json:"PreviousFields"`
	PreviousTxnID     string                 `json:"PreviousTxnID,omitempty"`
	PreviousTxnLgrSeq uint64                 `json:"PreviousTxnLgrSeq,omitempty"`
}

type AddressAmount struct {
	Address string
	Amount  *big.Int
}
