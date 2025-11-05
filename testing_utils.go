//go:build !test
// +build !test

package main

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/database/postgres"
	"github.com/lockplane/lockplane/database/sqlite"
)

// TestDB encapsulates a test database connection and driver
type TestDB struct {
	DB     *sql.DB
	Driver database.Driver
	Type   string
	ctx    context.Context
}

// Close closes the database connection
func (tdb *TestDB) Close() {
	if tdb.DB != nil {
		_ = tdb.DB.Close()
	}
}

// SetupTestDB creates a test database connection for the specified driver type
// Skips the test if the database is unavailable (unless REQUIRE_TEST_DB=true)
func SetupTestDB(t *testing.T, driverType string) *TestDB {
	t.Helper()

	requireDB := os.Getenv("REQUIRE_TEST_DB") == "true"

	var db *sql.DB
	var driver database.Driver
	var err error

	switch driverType {
	case "postgres", "postgresql":
		connStr := os.Getenv("POSTGRES_TEST_URL")
		if connStr == "" {
			connStr = "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
		}

		db, err = sql.Open("postgres", connStr)
		if err != nil {
			if requireDB {
				t.Fatalf("PostgreSQL required but unavailable: %v", err)
			}
			t.Skipf("PostgreSQL not available: %v", err)
		}

		if err := db.Ping(); err != nil {
			_ = db.Close()
			if requireDB {
				t.Fatalf("PostgreSQL required but unreachable: %v", err)
			}
			t.Skipf("PostgreSQL not reachable: %v", err)
		}

		driver = postgres.NewDriver()

	case "sqlite", "sqlite3":
		// Use in-memory database for fast tests
		db, err = sql.Open("sqlite", ":memory:")
		if err != nil {
			if requireDB {
				t.Fatalf("SQLite required but unavailable: %v", err)
			}
			t.Skipf("SQLite not available: %v", err)
		}

		// Enable foreign keys for SQLite
		_, err = db.Exec("PRAGMA foreign_keys = ON")
		if err != nil {
			_ = db.Close()
			t.Fatalf("Failed to enable foreign keys: %v", err)
		}

		driver = sqlite.NewDriver()

	case "libsql":
		// Use in-memory database for libSQL tests
		connStr := os.Getenv("LIBSQL_TEST_URL")
		if connStr == "" {
			connStr = "file::memory:?cache=shared"
		}

		db, err = sql.Open("libsql", connStr)
		if err != nil {
			if requireDB {
				t.Fatalf("libSQL required but unavailable: %v", err)
			}
			t.Skipf("libSQL not available: %v", err)
		}

		driver = sqlite.NewDriver() // libSQL uses SQLite driver

	default:
		t.Fatalf("Unknown database type: %s", driverType)
	}

	return &TestDB{
		DB:     db,
		Driver: driver,
		Type:   driverType,
		ctx:    context.Background(),
	}
}

// CleanupTables drops the specified tables (safe cleanup for tests)
func (tdb *TestDB) CleanupTables(t *testing.T, tables ...string) {
	t.Helper()

	for _, table := range tables {
		var sql string
		if tdb.Type == "postgres" || tdb.Type == "postgresql" {
			sql = "DROP TABLE IF EXISTS " + table + " CASCADE"
		} else {
			sql = "DROP TABLE IF EXISTS " + table
		}
		_, _ = tdb.DB.ExecContext(tdb.ctx, sql)
	}
}

// GetAllDrivers returns list of all supported drivers for parameterized tests
func GetAllDrivers() []string {
	drivers := []string{"postgres"}

	// Only test SQLite/libSQL if explicitly enabled
	// (to keep default test runs fast)
	if os.Getenv("TEST_ALL_DRIVERS") == "true" {
		drivers = append(drivers, "sqlite")
		if os.Getenv("TEST_LIBSQL") == "true" {
			drivers = append(drivers, "libsql")
		}
	}

	return drivers
}
