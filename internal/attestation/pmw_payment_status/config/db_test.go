package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitDBWithRetries(t *testing.T) {
	opts := &DBOptions{Retries: 3, RetryDelay: 1 * time.Millisecond, MaxDelay: 2 * time.Millisecond}
	DBOptionsName := "fakeDB"
	t.Run("SuccessFirstTry", func(t *testing.T) {
		db, err := initDBWithRetries(sqlite.Open(":memory:"), "test DB", opts)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer CloseDB(db)
	})
	t.Run("FailureExhaustRetries", func(t *testing.T) {
		db, err := initDBWithRetries(postgres.Open("invalid_dsn"), DBOptionsName, opts)
		require.Error(t, err)
		fmt.Print(err)
		require.Contains(t, err.Error(), DBOptionsName)
		require.Nil(t, db)
	})
	t.Run("BackoffStopsAtMaxDelay", func(t *testing.T) {
		start := time.Now()
		_, _ = initDBWithRetries(postgres.Open("invalid_dsn"), DBOptionsName, opts)
		elapsed := time.Since(start)

		expected := 3 * time.Millisecond
		require.GreaterOrEqual(t, elapsed, expected)
	})
}

func TestCloseDB(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		err = CloseDB(db)
		require.NoError(t, err)
	})
	t.Run("NilDB", func(t *testing.T) {
		err := CloseDB(nil)
		require.NoError(t, err)
	})
}

func TestInitMainAndCChainDB(t *testing.T) {
	testOpts := &DBOptions{
		Retries:    2,
		RetryDelay: 1 * time.Millisecond,
		MaxDelay:   2 * time.Millisecond,
	}

	t.Run("InitMainDB_InvalidDSN", func(t *testing.T) {
		db, err := InitMainDB("invalid_dsn", testOpts)
		require.Error(t, err)
		require.Nil(t, db)
		require.Contains(t, err.Error(), "main DB")
	})
	t.Run("InitCChainDB_InvalidDSN", func(t *testing.T) {
		db, err := InitCChainDB("invalid_dsn", testOpts)
		require.Error(t, err)
		require.Nil(t, db)
		require.Contains(t, err.Error(), "CChain DB")
	})
}
