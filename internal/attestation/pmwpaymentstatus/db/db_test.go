package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func TestInitDBWithRetries(t *testing.T) {
	opts := &DBOptions{MaxAttempts: 3, RetryDelay: 1 * time.Millisecond, MaxDelay: 2 * time.Millisecond}
	DBOptionsName := "fakeDB"
	t.Run("success", func(t *testing.T) {
		db, err := initDBWithRetries(sqlite.Open(":memory:"), "test DB", opts)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer func() { _ = CloseDB(db) }()
	})
	t.Run("exhaust retries", func(t *testing.T) {
		db, err := initDBWithRetries(postgres.Open("invalid_dsn"), DBOptionsName, opts)
		require.ErrorContains(t, err, "failed to open fakeDB after 3 attempts")
		require.NotContains(t, err.Error(), "invalid_dsn", "the connection string must not leak into the error")
		require.Nil(t, db)
	})
	t.Run("back off stop at max delay", func(t *testing.T) {
		start := time.Now()
		_, _ = initDBWithRetries(postgres.Open("invalid_dsn"), DBOptionsName, opts)
		elapsed := time.Since(start)

		expected := 3 * time.Millisecond
		require.GreaterOrEqual(t, elapsed, expected)
	})
}

// dsnEchoDialector is a gorm.Dialector whose connection fails with an error that
// embeds the whole DSN — the worst case a driver can produce. Only Name and
// Initialize are reached on the failure path; the rest must never be called.
type dsnEchoDialector struct{ dsn string }

func (dsnEchoDialector) Name() string { return "dsn-echo" }
func (d dsnEchoDialector) Initialize(*gorm.DB) error {
	return fmt.Errorf("connection refused for %s", d.dsn)
}

func (dsnEchoDialector) Migrator(*gorm.DB) gorm.Migrator                { panic("unused") }
func (dsnEchoDialector) DataTypeOf(*schema.Field) string                { panic("unused") }
func (dsnEchoDialector) DefaultValueOf(*schema.Field) clause.Expression { panic("unused") }
func (dsnEchoDialector) BindVarTo(clause.Writer, *gorm.Statement, any)  { panic("unused") }
func (dsnEchoDialector) QuoteTo(clause.Writer, string)                  { panic("unused") }
func (dsnEchoDialector) Explain(string, ...any) string                  { panic("unused") }

// A driver's connection error can embed the DSN, and a DSN carries credentials.
// The boundary returns a connection string-free message, so no part of the DSN
// reaches the caller's error — proven here with a dialector that echoes the
// whole DSN, deterministically and without a network.
func TestConnectErrorDoesNotLeakTheDSN(t *testing.T) {
	const dsn = "host=db user=app password=sup3r-s3cret dbname=verifier" //nolint:gosec // test credential
	opts := &DBOptions{MaxAttempts: 1, RetryDelay: time.Millisecond, MaxDelay: time.Millisecond}

	db, err := initDBWithRetries(dsnEchoDialector{dsn: dsn}, "Source DB", opts)
	require.Error(t, err)
	require.Nil(t, db)
	for _, leak := range []string{dsn, "password=sup3r-s3cret", "sup3r-s3cret", "user=app"} {
		require.NotContains(t, err.Error(), leak, "the DSN must not appear in the returned error")
	}
}

// noPoolDialector connects "successfully" but leaves no usable connection pool,
// so db.DB() fails — exercising the pool-configuration and CloseDB error paths.
type noPoolDialector struct{ dsnEchoDialector }

func (noPoolDialector) Initialize(*gorm.DB) error { return nil }

func TestPoolConfigAndCloseErrorsSurface(t *testing.T) {
	opts := &DBOptions{MaxAttempts: 1, RetryDelay: time.Millisecond, MaxDelay: time.Millisecond}

	_, err := initDBWithRetries(noPoolDialector{}, "Source DB", opts)
	require.ErrorContains(t, err, "cannot configure connection pool for Source DB")

	db, err := gorm.Open(noPoolDialector{}, &gorm.Config{})
	require.NoError(t, err)
	require.ErrorContains(t, CloseDB(db), "failed to obtain underlying sql.DB")
}

func TestCloseDB(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		err = CloseDB(db)
		require.NoError(t, err)
	})
	t.Run("nil db", func(t *testing.T) {
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
	t.Run("invalid source dsn", func(t *testing.T) {
		db, err := InitSourceDB("invalid_dsn", testOpts)
		require.ErrorContains(t, err, "failed to open Source DB after 2 attempts")
		require.NotContains(t, err.Error(), "invalid_dsn", "the DSN must not leak into the error")
		require.Nil(t, db)
	})
	t.Run("invalid cchain dsn", func(t *testing.T) {
		db, err := InitCChainDB("invalid_dsn", testOpts)
		require.ErrorContains(t, err, "failed to open CChain DB after 2 attempts")
		require.NotContains(t, err.Error(), "invalid_dsn", "the DSN must not leak into the error")
		require.Nil(t, db)
	})
}
