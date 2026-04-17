package db

import (
	"fmt"
	"strings"
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

	dbMaxOpenConns    = 25
	dbMaxIdleConns    = 10
	dbConnMaxLifetime = 5 * time.Minute
	dbConnMaxIdleTime = 5 * time.Minute
)

type DBOptions struct {
	MaxAttempts int
	RetryDelay  time.Duration
	MaxDelay    time.Duration
}

func initDBWithRetries(dialector gorm.Dialector, dsn, dbName string, opts *DBOptions) (*gorm.DB, error) {
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
			if poolErr := configurePool(db); poolErr != nil {
				return nil, fmt.Errorf("cannot configure connection pool for %s: %w", dbName, poolErr)
			}
			logger.Infof("Successfully connected to %s (%s) on attempt %d", dbName, dialector.Name(), attempt)
			return db, nil
		}
		logger.Warnf("Attempt %d: failed to connect to %s (%s): %s", attempt, dbName, dialector.Name(), redactDSN(err, dsn))

		if attempt < maxAttempts {
			logger.Infof("Retrying in %v...", currentDelay)
			time.Sleep(currentDelay)
			currentDelay *= 2
			if currentDelay > maxDelay {
				currentDelay = maxDelay
			}
		}
	}
	return nil, fmt.Errorf("failed to open %s after %d attempts: %s", dbName, maxAttempts, redactDSN(err, dsn))
}

// redactDSN replaces any occurrence of the raw DSN in an error message with
// [redacted] so that credentials embedded in connection strings are not written
// to logs.
func redactDSN(err error, dsn string) string {
	return strings.ReplaceAll(err.Error(), dsn, "[redacted]")
}

func configurePool(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(dbMaxOpenConns)
	sqlDB.SetMaxIdleConns(dbMaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(dbConnMaxIdleTime)
	return nil
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
	return initDBWithRetries(postgres.Open(dsn), dsn, "Source DB", opts)
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
	return initDBWithRetries(mysql.Open(dsn), dsn, "CChain DB", opts)
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
