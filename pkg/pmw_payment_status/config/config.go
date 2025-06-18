package config

import (
	"fmt"
	"os"
	"sync"
)

var (
	sourceID          string
	cchainDatabaseURL string
	databaseURL       string
	once              sync.Once
	initErr           error
)

func loadEnv() {
	sourceID = os.Getenv("SOURCE_ID")
	cchainDatabaseURL = os.Getenv("CCHAIN_DATABASE_URL")
	databaseURL = os.Getenv("DATABASE_URL")

	if sourceID == "" {
		initErr = fmt.Errorf("SOURCE_ID not set")
		return
	}
	if len(sourceID) > 32 {
		initErr = fmt.Errorf("SOURCE_ID longer than 32 bytes")
		return
	}
	if cchainDatabaseURL == "" {
		initErr = fmt.Errorf("CCHAIN_DATABASE_URL not set")
		return
	}
	if databaseURL == "" {
		initErr = fmt.Errorf("DATABASE_URL not set")
		return
	}
}

func Init() error {
	once.Do(loadEnv)
	return initErr
}

func CchainDatabaseURL() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	return cchainDatabaseURL, nil
}

func DatabaseURL() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	return databaseURL, nil
}

func SourceID() (string, error) {
	if err := Init(); err != nil {
		return "", err
	}
	return sourceID, nil
}
