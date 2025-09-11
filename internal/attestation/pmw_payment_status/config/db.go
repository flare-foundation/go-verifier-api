package config

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
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
		Retries:    5,
		RetryDelay: 1 * time.Second,
		MaxDelay:   5 * time.Second,
	}
	return initDBWithRetries(postgres.Open(dsn), "main DB", opts)
}

func InitCChainDB(dsn string) (*gorm.DB, error) {
	opts := &DBOptions{
		Retries:    10,
		RetryDelay: 2 * time.Second,
		MaxDelay:   10 * time.Second,
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
