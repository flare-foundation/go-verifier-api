package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithPostgresStatementTimeout(t *testing.T) {
	t.Run("appends to a URL DSN with an existing query", func(t *testing.T) {
		got := withPostgresStatementTimeout("postgres://u:p@h:5432/db?sslmode=disable")
		require.Equal(t, "postgres://u:p@h:5432/db?sslmode=disable&statement_timeout=28000", got)
	})

	t.Run("appends to a URL DSN with no query", func(t *testing.T) {
		got := withPostgresStatementTimeout("postgres://u:p@h:5432/db")
		require.Equal(t, "postgres://u:p@h:5432/db?statement_timeout=28000", got)
	})

	t.Run("appends to a keyword-form DSN", func(t *testing.T) {
		got := withPostgresStatementTimeout("host=h port=5432 user=u dbname=db")
		require.Equal(t, "host=h port=5432 user=u dbname=db statement_timeout=28000", got)
	})

	t.Run("respects an operator-provided value", func(t *testing.T) {
		dsn := "postgres://u:p@h:5432/db?statement_timeout=5000"
		require.Equal(t, dsn, withPostgresStatementTimeout(dsn))
	})

	t.Run("respects an operator-provided value at the start of a keyword DSN", func(t *testing.T) {
		dsn := "statement_timeout=5000 host=h port=5432 dbname=db"
		require.Equal(t, dsn, withPostgresStatementTimeout(dsn))
	})

	t.Run("does not false-match the key inside a value", func(t *testing.T) {
		// The key appears in the password, not as a parameter, so the real
		// statement_timeout must still be appended.
		got := withPostgresStatementTimeout("postgres://u:statement_timeout=x@h:5432/db")
		require.Equal(t, "postgres://u:statement_timeout=x@h:5432/db?statement_timeout=28000", got)
	})
}

func TestWithMySQLIOTimeouts(t *testing.T) {
	t.Run("appends read and write timeouts", func(t *testing.T) {
		got := withMySQLIOTimeouts("u:p@tcp(h:3306)/db?parseTime=true")
		require.Equal(t, "u:p@tcp(h:3306)/db?parseTime=true&readTimeout=28s&writeTimeout=28s", got)
	})

	t.Run("adds the query separator when none is present", func(t *testing.T) {
		got := withMySQLIOTimeouts("u:p@tcp(h:3306)/db")
		require.Equal(t, "u:p@tcp(h:3306)/db?readTimeout=28s&writeTimeout=28s", got)
	})

	t.Run("respects operator-provided values", func(t *testing.T) {
		dsn := "u:p@tcp(h:3306)/db?readTimeout=1s&writeTimeout=2s"
		require.Equal(t, dsn, withMySQLIOTimeouts(dsn))
	})
}
