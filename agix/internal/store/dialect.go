package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Dialect represents the SQL database backend.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// DetectDialect returns the dialect based on the DSN string.
// If the DSN starts with "postgres://" or "postgresql://", it returns DialectPostgres.
// Otherwise, it returns DialectSQLite (treating the DSN as a file path).
func DetectDialect(dsn string) Dialect {
	lower := strings.ToLower(dsn)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return DialectPostgres
	}
	return DialectSQLite
}

// OpenDB opens a database connection for the given DSN.
// For SQLite, it appends WAL mode and busy timeout pragmas.
// For PostgreSQL, it uses the DSN directly.
func OpenDB(dsn string) (*sql.DB, Dialect, error) {
	dialect := DetectDialect(dsn)

	switch dialect {
	case DialectPostgres:
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, dialect, fmt.Errorf("open postgres: %w", err)
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, dialect, fmt.Errorf("ping postgres: %w", err)
		}
		return db, dialect, nil
	default:
		db, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			return nil, dialect, fmt.Errorf("open sqlite: %w", err)
		}
		return db, dialect, nil
	}
}

// Rebind rewrites a query with `?` placeholders to use `$1, $2, ...` for PostgreSQL.
// For SQLite, it returns the query unchanged.
func Rebind(dialect Dialect, query string) string {
	if dialect == DialectSQLite {
		return query
	}

	var out strings.Builder
	n := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			out.WriteByte('$')
			out.WriteString(strconv.Itoa(n))
			n++
		} else {
			out.WriteByte(query[i])
		}
	}
	return out.String()
}
