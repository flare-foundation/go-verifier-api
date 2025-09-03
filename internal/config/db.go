package config

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	defaultDBOpenRetries = 3
	defaultDBRetryDelay  = 500 * time.Millisecond
)

func InitMainDB(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for i := 0; i < defaultDBOpenRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			return db, nil
		}
		time.Sleep(defaultDBRetryDelay)
	}
	return nil, fmt.Errorf("failed to open main DB after %d attempts: %w", defaultDBOpenRetries, err)
}

func InitCChainDB(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for i := 0; i < defaultDBOpenRetries; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			return db, nil
		}
		time.Sleep(defaultDBRetryDelay)
	}
	return nil, fmt.Errorf("failed to open CChain DB after %d attempts: %w", defaultDBOpenRetries, err)
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
