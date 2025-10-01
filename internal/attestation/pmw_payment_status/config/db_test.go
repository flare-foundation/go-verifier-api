package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitDBWithRetries(t *testing.T) {
	opts := &DBOptions{MaxAttempts: 3, RetryDelay: 1 * time.Millisecond, MaxDelay: 2 * time.Millisecond}
	DBOptionsName := "fakeDB"
	t.Run("SuccessFirstTry", func(t *testing.T) {
		db, err := initDBWithRetries(sqlite.Open(":memory:"), "test DB", opts)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer func() { _ = CloseDB(db) }()
	})
	t.Run("FailureExhaustRetries", func(t *testing.T) {
		db, err := initDBWithRetries(postgres.Open("invalid_dsn"), DBOptionsName, opts)
		require.ErrorContains(t, err, "failed to open fakeDB after 3 attempts: cannot parse `invalid_dsn`")
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
		MaxAttempts: 2,
		RetryDelay:  1 * time.Millisecond,
		MaxDelay:    2 * time.Millisecond,
	}
	t.Run("InitMainDB_InvalidDSN", func(t *testing.T) {
		db, err := InitMainDB("invalid_dsn", testOpts)
		require.ErrorContains(t, err, "failed to open main DB after 2 attempts: cannot parse `invalid_dsn`: failed to parse")
		require.Nil(t, db)
	})
	t.Run("InitCChainDB_InvalidDSN", func(t *testing.T) {
		db, err := InitCChainDB("invalid_dsn", testOpts)
		require.ErrorContains(t, err, "failed to open CChain DB after 2 attempts: invalid DSN: missing the slash")
		require.Nil(t, db)
	})
}
