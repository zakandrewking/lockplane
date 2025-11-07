package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/lockplane/lockplane/internal/schema"
)

// isConnectionString checks if a string looks like a database connection string
func isConnectionString(s string) bool {
	lower := strings.ToLower(s)

	// Check for common connection string prefixes
	if strings.HasPrefix(lower, "postgres://") ||
		strings.HasPrefix(lower, "postgresql://") ||
		strings.HasPrefix(lower, "libsql://") ||
		strings.HasPrefix(lower, "sqlite://") ||
		strings.HasPrefix(lower, "file:") {
		return true
	}

	// Check if it looks like a SQLite file path that doesn't exist as a regular file
	// If the file exists, we'll let LoadSchema handle it
	if strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".sqlite3") {
		// Check if the file exists using original path (not lowercased) - if it does, it's a file path
		if _, err := os.Stat(s); err == nil {
			return false
		}
		// If it doesn't exist as a file, treat it as a potential connection string
		return true
	}

	// :memory: is a SQLite in-memory database
	if lower == ":memory:" {
		return true
	}

	return false
}

// loadSchemaFromConnectionString introspects a database and returns its schema
func loadSchemaFromConnectionString(connStr string) (*Schema, error) {
	// Detect database driver from connection string
	driverType := detectDriver(connStr)
	driver, err := newDriver(driverType)
	if err != nil {
		return nil, fmt.Errorf("failed to create database driver: %w", err)
	}

	// Get the SQL driver name (use detected type, not driver.Name())
	sqlDriverName := getSQLDriverName(driverType)

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

	result := (*Schema)(dbSchema)
	result.Dialect = schema.DriverNameToDialect(driverType)
	return result, nil
}

// LoadSchemaOrIntrospect loads a schema from a file/directory or introspects from a database connection string
func LoadSchemaOrIntrospect(pathOrConnStr string) (*Schema, error) {
	return LoadSchemaOrIntrospectWithOptions(pathOrConnStr, nil)
}

// LoadSchemaOrIntrospectWithOptions loads a schema with optional parsing options.
func LoadSchemaOrIntrospectWithOptions(pathOrConnStr string, opts *schema.SchemaLoadOptions) (*Schema, error) {
	// Check if it's a connection string
	if isConnectionString(pathOrConnStr) {
		return loadSchemaFromConnectionString(pathOrConnStr)
	}

	// Otherwise treat it as a file path
	dbSchema, err := schema.LoadSchemaWithOptions(pathOrConnStr, opts)
	if err != nil {
		return nil, err
	}
	return (*Schema)(dbSchema), nil
}
