package config

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"time"

	"gorm.io/gorm"
)

const (
	defaultDBOpenRetries = 3
	defaultDBRetryDelay  = 500 * time.Millisecond
)

func initDBWithRetries(dialector gorm.Dialector, dbName string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	for i := 0; i < defaultDBOpenRetries; i++ {
		db, err = gorm.Open(dialector, &gorm.Config{})
		if err == nil {
			return db, nil
		}

		if i < defaultDBOpenRetries-1 {
			time.Sleep(defaultDBRetryDelay)
		}
	}

	return nil, fmt.Errorf("failed to open %s after %d attempts: %w", dbName, defaultDBOpenRetries, err)
}

func InitMainDB(dsn string) (*gorm.DB, error) {
	return initDBWithRetries(postgres.Open(dsn), "main DB")
}

func InitCChainDB(dsn string) (*gorm.DB, error) {
	return initDBWithRetries(mysql.Open(dsn), "CChain DB")
}

func CloseGormDB(db *gorm.DB) error {
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
