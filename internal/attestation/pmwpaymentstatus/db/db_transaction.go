package db

import (
	"fmt"
	"strings"
)

// CheckRowConsistency verifies that the identity fields parsed from a row's
// Response JSON (hash/account/sequence) agree with the canonical DB columns.
// Protects against partial writes or targeted tampering of a subset of a row's
// fields; does not protect against a fully compromised indexer (acknowledged
// trust boundary, see L-02 in audit.md). Shared by PMWPaymentStatus and
// PMWFeeProof so both bind the Response blob to the row before trusting it.
func CheckRowConsistency(jsonHash, jsonAccount string, jsonSequence uint64, tx DBTransaction) error {
	if !strings.EqualFold(jsonHash, tx.Hash) {
		return fmt.Errorf("DB inconsistency: JSON hash %q != column Hash %q: %w", jsonHash, tx.Hash, ErrDatabase)
	}
	if jsonAccount != tx.SourceAddress {
		return fmt.Errorf("DB inconsistency: JSON Account %q != column SourceAddress %q: %w", jsonAccount, tx.SourceAddress, ErrDatabase)
	}
	if jsonSequence != tx.Sequence {
		return fmt.Errorf("DB inconsistency: JSON Sequence %d != column Sequence %d: %w", jsonSequence, tx.Sequence, ErrDatabase)
	}
	return nil
}

// DBTransaction is a subset of verifier-xrp-indexer's internal/xrp.Transaction
// (cannot import due to Go internal package visibility). Only fields used by the
// verifier are included; GORM silently ignores extra DB columns.
type DBTransaction struct {
	Hash                string `gorm:"primaryKey;type:varchar(64)"`
	BlockNumber         uint64 `gorm:"index"`
	Timestamp           uint64 `gorm:"index"`
	PaymentReference    string `gorm:"index;type:varchar(64);default:null"`
	Response            string `gorm:"type:varchar"`
	IsNativePayment     bool   `gorm:"index"`
	SourceAddressesRoot string `gorm:"index;type:varchar(64);default:null"`
	Sequence            uint64 `gorm:"index:idx_source_sequence,priority:2"`
	TicketSequence      uint64
	SourceAddress       string `gorm:"index:idx_source_sequence,priority:1;type:varchar(64)"`
}

func (DBTransaction) TableName() string {
	return "transactions"
}
