package db

import (
	"fmt"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
)

const (
	mainDBMaxAttempts = 5
	mainDBRetryDelay  = 1 * time.Second
	mainDBMaxDelay    = 5 * time.Second

	cChainDBMaxAttempts = 10
	cChainDBRetryDelay  = 2 * time.Second
	cChainDBMaxDelay    = 10 * time.Second
)

type DBOptions struct {
	MaxAttempts int
	RetryDelay  time.Duration
	MaxDelay    time.Duration
}

func initDBWithRetries(dialector gorm.Dialector, dbName string, opts *DBOptions) (*gorm.DB, error) {
	maxAttempts := opts.MaxAttempts
	delay := opts.RetryDelay
	maxDelay := opts.MaxDelay

	var db *gorm.DB
	var err error
	currentDelay := delay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Infof("Attempt %d: connecting to %s (%s)", attempt, dbName, dialector.Name())
		db, err = gorm.Open(dialector, &gorm.Config{})
		if err == nil {
			logger.Infof("Successfully connected to %s (%s) on attempt %d", dbName, dialector.Name(), attempt)
			return db, nil
		}
		logger.Warnf("Attempt %d: failed to connect to %s (%s): %v", attempt, dbName, dialector.Name(), err)

		if attempt < maxAttempts {
			logger.Infof("Retrying in %v...", currentDelay)
			time.Sleep(currentDelay)
			currentDelay *= 2
			if currentDelay > maxDelay {
				currentDelay = maxDelay
			}
		}
	}
	return nil, fmt.Errorf("failed to open %s after %d attempts: %w", dbName, maxAttempts, err)
}

func InitSourceDB(dsn string, overrideOpts *DBOptions) (*gorm.DB, error) {
	opts := &DBOptions{
		MaxAttempts: mainDBMaxAttempts,
		RetryDelay:  mainDBRetryDelay,
		MaxDelay:    mainDBMaxDelay,
	}
	if overrideOpts != nil {
		opts = overrideOpts
	}
	return initDBWithRetries(postgres.Open(dsn), "Source DB", opts)
}

func InitCChainDB(dsn string, overrideOpts *DBOptions) (*gorm.DB, error) {
	opts := &DBOptions{
		MaxAttempts: cChainDBMaxAttempts,
		RetryDelay:  cChainDBRetryDelay,
		MaxDelay:    cChainDBMaxDelay,
	}
	if overrideOpts != nil {
		opts = overrideOpts
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
