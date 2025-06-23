package pmwpaymentstatusconfig

import (
	"fmt"
	"os"
)

type PMWPaymentStatusConfig struct {
	SourceID          string
	DatabaseURL       string
	CchainDatabaseURL string
}

func LoadPMWPaymentStatusConfig() (*PMWPaymentStatusConfig, error) {
	sourceID := os.Getenv("SOURCE_ID")
	if sourceID == "" {
		return nil, fmt.Errorf("SOURCE_ID not set")
	}
	if len(sourceID) > 32 {
		return nil, fmt.Errorf("SOURCE_ID longer than 32 bytes")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}
	cChainDbURL := os.Getenv("CCHAIN_DATABASE_URL")
	if cChainDbURL == "" {
		return nil, fmt.Errorf("CCHAIN_DATABASE_URL not set")
	}

	return &PMWPaymentStatusConfig{
		SourceID:          sourceID,
		DatabaseURL:       dbURL,
		CchainDatabaseURL: cChainDbURL,
	}, nil
}
