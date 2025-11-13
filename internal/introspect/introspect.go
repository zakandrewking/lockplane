package introspect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/sqliteutil"
)

// IsConnectionString checks if a string looks like a database connection string
func IsConnectionString(s string) bool {
	lower := strings.ToLower(s)

	// Check for common connection string prefixes
	if strings.HasPrefix(lower, "postgres://") ||
		strings.HasPrefix(lower, "postgresql://") ||
		strings.HasPrefix(lower, "libsql://") ||
		strings.HasPrefix(lower, "sqlite://") ||
		strings.HasPrefix(lower, "file:") {
		return true
	}

	// Check if it looks like a SQLite file path
	// Always treat .db files as SQLite databases to introspect, not JSON files
	if strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".sqlite3") {
		return true
	}

	// :memory: is a SQLite in-memory database
	if lower == ":memory:" {
		return true
	}

	return false
}

// LoadSchemaFromConnectionString introspects a database and returns its schema
func LoadSchemaFromConnectionString(connStr string, detectDriverFunc func(string) string, newDriverFunc func(string) (database.Driver, error), getSQLDriverNameFunc func(string) string) (*database.Schema, error) {
	// Detect database driver from connection string
	driverType := detectDriverFunc(connStr)

	// For SQLite, check if the database file exists and create it if needed
	// Also offer to create shadow database
	if driverType == "sqlite" || driverType == "sqlite3" {
		if err := sqliteutil.EnsureSQLiteDatabaseWithShadow(connStr, "target", false, true); err != nil {
			return nil, err
		}
	}

	driver, err := newDriverFunc(driverType)
	if err != nil {
		return nil, fmt.Errorf("failed to create database driver: %w", err)
	}

	// Get the SQL driver name (use detected type, not driver.Name())
	sqlDriverName := getSQLDriverNameFunc(driverType)

	db, err := sql.Open(sqlDriverName, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	dbSchema, err := driver.IntrospectSchema(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect schema: %w", err)
	}

	result := dbSchema
	result.Dialect = schema.DriverNameToDialect(driverType)
	return result, nil
}

// LoadSchemaOrIntrospect loads a schema from a file/directory or introspects from a database connection string
func LoadSchemaOrIntrospect(pathOrConnStr string, detectDriverFunc func(string) string, newDriverFunc func(string) (database.Driver, error), getSQLDriverNameFunc func(string) string) (*database.Schema, error) {
	return LoadSchemaOrIntrospectWithOptions(pathOrConnStr, nil, detectDriverFunc, newDriverFunc, getSQLDriverNameFunc)
}

// LoadSchemaOrIntrospectWithOptions loads a schema with optional parsing options.
func LoadSchemaOrIntrospectWithOptions(pathOrConnStr string, opts *schema.SchemaLoadOptions, detectDriverFunc func(string) string, newDriverFunc func(string) (database.Driver, error), getSQLDriverNameFunc func(string) string) (*database.Schema, error) {
	// Check if it's a connection string
	if IsConnectionString(pathOrConnStr) {
		return LoadSchemaFromConnectionString(pathOrConnStr, detectDriverFunc, newDriverFunc, getSQLDriverNameFunc)
	}

	// Otherwise treat it as a file path
	dbSchema, err := schema.LoadSchemaWithOptions(pathOrConnStr, opts)
	if err != nil {
		return nil, err
	}
	return dbSchema, nil
}
