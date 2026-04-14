package db

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
