package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lockplane/lockplane/database"
	"github.com/xeipuuv/gojsonschema"
)

// SchemaLoadOptions controls how schema files are parsed.
type SchemaLoadOptions struct {
	Dialect database.Dialect
}

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

	schema, err := driver.IntrospectSchema(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect schema: %w", err)
	}

	result := (*Schema)(schema)
	result.Dialect = driverNameToDialect(driverType)
	return result, nil
}

// LoadSchemaOrIntrospect loads a schema from a file/directory or introspects from a database connection string
func LoadSchemaOrIntrospect(pathOrConnStr string) (*Schema, error) {
	return LoadSchemaOrIntrospectWithOptions(pathOrConnStr, nil)
}

// LoadSchemaOrIntrospectWithOptions loads a schema with optional parsing options.
func LoadSchemaOrIntrospectWithOptions(pathOrConnStr string, opts *SchemaLoadOptions) (*Schema, error) {
	// Check if it's a connection string
	if isConnectionString(pathOrConnStr) {
		return loadSchemaFromConnectionString(pathOrConnStr)
	}

	// Otherwise treat it as a file path
	return LoadSchemaWithOptions(pathOrConnStr, opts)
}

// LoadSchema loads a schema from either JSON (.json) or SQL DDL (.lp.sql) file
func LoadSchema(path string) (*Schema, error) {
	return LoadSchemaWithOptions(path, nil)
}

// LoadSchemaWithOptions loads a schema with optional parsing options.
func LoadSchemaWithOptions(path string, opts *SchemaLoadOptions) (*Schema, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return loadSchemaFromDir(path, opts)
	}

	ext := strings.ToLower(filepath.Ext(path))

	// Check for .lp.sql extension
	if ext == ".sql" && strings.HasSuffix(strings.ToLower(path), ".lp.sql") {
		return LoadSQLSchemaWithOptions(path, opts)
	}

	// Otherwise assume JSON
	return LoadJSONSchema(path)
}

// LoadSQLSchema loads a schema from a SQL DDL file
func LoadSQLSchema(path string) (*Schema, error) {
	return LoadSQLSchemaWithOptions(path, nil)
}

// LoadSQLSchemaWithOptions loads a SQL schema with optional parsing options.
func LoadSQLSchemaWithOptions(path string, opts *SchemaLoadOptions) (*Schema, error) {
	// Read the SQL file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read SQL file: %w", err)
	}

	return loadSQLSchemaFromBytes(data, opts)
}

func loadSQLSchemaFromBytes(data []byte, opts *SchemaLoadOptions) (*Schema, error) {
	dialect := database.DialectUnknown
	if opts != nil && opts.Dialect != database.DialectUnknown {
		dialect = opts.Dialect
	}
	if dialect == database.DialectUnknown {
		dialect = detectDialectFromSQL(data)
	}
	if dialect == database.DialectUnknown {
		dialect = database.DialectPostgres
	}

	// Parse SQL DDL
	schema, err := ParseSQLSchemaWithDialect(string(data), dialect)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL DDL: %w", err)
	}

	// Convert from database.Schema to main.Schema (they're type aliases, so just cast)
	result := (*Schema)(schema)
	result.Dialect = dialect
	return result, nil
}

func loadSchemaFromDir(dir string, opts *SchemaLoadOptions) (*Schema, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema directory %s: %w", dir, err)
	}

	var sqlFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		if strings.HasSuffix(strings.ToLower(entry.Name()), ".lp.sql") {
			sqlFiles = append(sqlFiles, filepath.Join(dir, entry.Name()))
		}
	}

	if len(sqlFiles) == 0 {
		return nil, fmt.Errorf("no .lp.sql files found in directory %s", dir)
	}

	sort.Strings(sqlFiles)

	var builder strings.Builder
	for _, file := range sqlFiles {
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read SQL file %s: %w", file, readErr)
		}

		builder.WriteString(fmt.Sprintf("-- File: %s\n", file))
		builder.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	}

	return loadSQLSchemaFromBytes([]byte(builder.String()), opts)
}

func detectDialectFromSQL(data []byte) database.Dialect {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "--") {
			lower := strings.ToLower(strings.TrimPrefix(trimmed, "--"))
			lower = strings.TrimSpace(lower)

			if strings.HasPrefix(lower, "dialect:") {
				value := strings.TrimSpace(strings.TrimPrefix(lower, "dialect:"))
				return parseDialect(value)
			}
			if strings.HasPrefix(lower, "dialect") {
				value := strings.TrimSpace(strings.TrimPrefix(lower, "dialect"))
				if strings.HasPrefix(value, ":") {
					value = strings.TrimSpace(strings.TrimPrefix(value, ":"))
				}
				return parseDialect(value)
			}
			continue
		}

		// Stop scanning when we hit a non-comment statement.
		break
	}
	return database.DialectUnknown
}

func parseDialect(value string) database.Dialect {
	switch strings.ToLower(value) {
	case "postgres", "postgresql":
		return database.DialectPostgres
	case "sqlite", "sqlite3", "libsql":
		return database.DialectSQLite
	default:
		return database.DialectUnknown
	}
}

func driverNameToDialect(name string) database.Dialect {
	switch strings.ToLower(name) {
	case "postgres", "postgresql":
		return database.DialectPostgres
	case "sqlite", "sqlite3", "libsql":
		return database.DialectSQLite
	default:
		return database.DialectUnknown
	}
}

// LoadJSONSchema loads and validates a JSON schema file, returning a Schema
func LoadJSONSchema(path string) (*Schema, error) {
	// Read the JSON file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse into Schema
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	// Validate against JSON Schema
	schemaLoader := gojsonschema.NewReferenceLoader("file://schema-json/schema.json")
	documentLoader := gojsonschema.NewStringLoader(string(data))

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		// If schema file doesn't exist, skip validation (backwards compatibility)
		return &schema, nil
	}

	if !result.Valid() {
		errMsg := "JSON Schema validation failed:\n"
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return &schema, nil
}

// ValidateJSONSchema validates a JSON file without loading it
func ValidateJSONSchema(path string) error {
	_, err := LoadJSONSchema(path)
	return err
}

// LoadJSONPlan loads and validates a JSON plan file, returning a Plan
func LoadJSONPlan(path string) (*Plan, error) {
	// Read the JSON file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse into Plan
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// Validate against JSON Schema
	schemaLoader := gojsonschema.NewReferenceLoader("file://schema-json/plan.json")
	documentLoader := gojsonschema.NewStringLoader(string(data))

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		// If schema file doesn't exist, skip validation (backwards compatibility)
		return &plan, nil
	}

	if !result.Valid() {
		errMsg := "JSON Schema validation failed:\n"
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return &plan, nil
}
