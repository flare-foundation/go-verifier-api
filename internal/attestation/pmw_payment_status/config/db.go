package config

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
)

const (
	mainDBRetries    = 5
	mainDBRetryDelay = 1 * time.Second
	mainDBMaxDelay   = 5 * time.Second

	cChainDBRetries    = 10
	cChainDBRetryDelay = 2 * time.Second
	cChainDBMaxDelay   = 10 * time.Second
)

type DBOptions struct {
	Retries    int
	RetryDelay time.Duration
	MaxDelay   time.Duration
}

func initDBWithRetries(dialector gorm.Dialector, dbName string, opts *DBOptions) (*gorm.DB, error) {
	retries := opts.Retries
	delay := opts.RetryDelay
	maxDelay := opts.MaxDelay

	var db *gorm.DB
	var err error
	currentDelay := delay

	for i := 0; i < retries; i++ {
		db, err = gorm.Open(dialector, &gorm.Config{})
		if err == nil {
			return db, nil
		}

		if i < retries-1 {
			time.Sleep(currentDelay)
			currentDelay *= 2
			if currentDelay > maxDelay {
				currentDelay = maxDelay
			}
		}
	}

	return nil, fmt.Errorf("failed to open %s after %d attempts: %w", dbName, retries, err)
}

func InitMainDB(dsn string) (*gorm.DB, error) {
	opts := &DBOptions{
		Retries:    mainDBRetries,
		RetryDelay: mainDBRetryDelay,
		MaxDelay:   mainDBMaxDelay,
	}
	return initDBWithRetries(postgres.Open(dsn), "main DB", opts)
}

func InitCChainDB(dsn string) (*gorm.DB, error) {
	opts := &DBOptions{
		Retries:    cChainDBRetries,
		RetryDelay: cChainDBRetryDelay,
		MaxDelay:   cChainDBMaxDelay,
	}
	return initDBWithRetries(mysql.Open(dsn), "CChain DB", opts)
}

func CloseDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to obtain underlying sql.DB: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}
