package db

import (
	"fmt"
	"strconv"
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

	// dbStatementTimeout bounds how long a single DB operation may run, as
	// defense-in-depth behind the per-request context deadline (verifierWorkTimeout,
	// the primary bound, which fires first). It is enforced differently per driver:
	// Postgres applies it as a true server-side statement_timeout; MySQL applies it
	// as read/write I/O timeouts (go-sql-driver has no portable server-side
	// statement cap). Kept above the 25s request deadline so it never pre-empts a
	// legitimately in-progress query, and below the 30s server writeTimeout so a
	// backstop abort still leaves margin to write a 503.
	dbStatementTimeout = 28 * time.Second
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

// dsnHasParam reports whether key already appears as a parameter in dsn, matching
// only at a parameter boundary (a leading `?`/`&`/space, or the very start of the
// DSN for a keyword-form param) so the key embedded in a value (e.g. a password)
// does not false-match. Properly-encoded DSNs never carry a bare `?`/`&`/space
// inside a value, so the boundary set is sufficient.
func dsnHasParam(dsn, key string) bool {
	if strings.HasPrefix(dsn, key+"=") {
		return true
	}
	for _, sep := range []string{"?", "&", " "} {
		if strings.Contains(dsn, sep+key+"=") {
			return true
		}
	}
	return false
}

// appendDSNParam adds key=value to a URL-style DSN query string unless the key is
// already present (operator override wins). Used for both the Postgres URL DSN and
// the go-sql-driver MySQL DSN, which share the ?key=value&... query syntax.
func appendDSNParam(dsn, key, value string) string {
	if dsnHasParam(dsn, key) {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + key + "=" + value
}

// withPostgresStatementTimeout sets pgx's per-session statement_timeout (a GUC, in
// milliseconds) so the server aborts any statement that runs past the cap. pgx
// accepts it in both DSN forms: as a query parameter in URL DSNs
// (postgres://...?statement_timeout=...) and as a space-separated key in
// keyword/DSN form (host=... statement_timeout=...). An operator-supplied value
// is left untouched.
func withPostgresStatementTimeout(dsn string) string {
	if dsnHasParam(dsn, "statement_timeout") {
		return dsn
	}
	ms := strconv.FormatInt(dbStatementTimeout.Milliseconds(), 10)
	if strings.Contains(dsn, "://") {
		return appendDSNParam(dsn, "statement_timeout", ms)
	}
	return dsn + " statement_timeout=" + ms
}

// withMySQLIOTimeouts sets go-sql-driver read/write I/O timeouts so a query whose
// results stall mid-stream cannot block a connection indefinitely. These are
// driver parameters (flavor-independent), unlike server session variables.
func withMySQLIOTimeouts(dsn string) string {
	d := dbStatementTimeout.String()
	dsn = appendDSNParam(dsn, "readTimeout", d)
	dsn = appendDSNParam(dsn, "writeTimeout", d)
	return dsn
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
	dsn = withPostgresStatementTimeout(dsn)
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
	dsn = withMySQLIOTimeouts(dsn)
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
