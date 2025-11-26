package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/parser"
	"github.com/xeipuuv/gojsonschema"
)

// SchemaLoadOptions controls how schema files are parsed.
type SchemaLoadOptions struct {
	Dialect database.Dialect
}

// LoadSchema loads a schema from either JSON (.json) or SQL DDL (.lp.sql) file
func LoadSchema(path string) (*database.Schema, error) {
	return LoadSchemaWithOptions(path, nil)
}

// LoadSchemaWithOptions loads a schema with optional parsing options.
func LoadSchemaWithOptions(path string, opts *SchemaLoadOptions) (*database.Schema, error) {
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
func LoadSQLSchema(path string) (*database.Schema, error) {
	return LoadSQLSchemaWithOptions(path, nil)
}

// LoadSQLSchemaWithOptions loads a SQL schema with optional parsing options.
func LoadSQLSchemaWithOptions(path string, opts *SchemaLoadOptions) (*database.Schema, error) {
	// Read the SQL file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read SQL file: %w", err)
	}

	return LoadSQLSchemaFromBytes(data, opts)
}

// LoadSQLSchemaFromBytes loads a SQL schema from a byte slice
func LoadSQLSchemaFromBytes(data []byte, opts *SchemaLoadOptions) (*database.Schema, error) {
	// Precedence order (most to least specific):
	// 1. CLI/config flag (opts.Dialect)
	// 2. Auto-detect from connection string (handled by callers)
	// 3. Default to Postgres

	dialect := database.DialectPostgres
	if opts != nil && opts.Dialect != database.DialectUnknown {
		dialect = opts.Dialect
	}

	// Parse SQL DDL
	schema, err := parser.ParseSQLSchemaWithDialect(string(data), dialect)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL DDL: %w", err)
	}

	schema.Dialect = dialect
	return schema, nil
}

func loadSchemaFromDir(dir string, opts *SchemaLoadOptions) (*database.Schema, error) {
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

		name := entry.Name()
		lowerName := strings.ToLower(name)

		// Skip database files
		if strings.HasSuffix(lowerName, ".db") ||
			strings.HasSuffix(lowerName, ".sqlite") ||
			strings.HasSuffix(lowerName, ".sqlite3") {
			continue
		}

		// Only include .lp.sql files
		if strings.HasSuffix(lowerName, ".lp.sql") {
			sqlFiles = append(sqlFiles, filepath.Join(dir, name))
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

	return LoadSQLSchemaFromBytes([]byte(builder.String()), opts)
}

// DriverNameToDialect converts a driver name to a dialect
func DriverNameToDialect(name string) database.Dialect {
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
func LoadJSONSchema(path string) (*database.Schema, error) {
	// Read the JSON file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Step 1: Validate against JSON Schema FIRST (catches extra fields via additionalProperties: false)
	schemaLoader := gojsonschema.NewReferenceLoader("file://schema-json/schema.json")
	documentLoader := gojsonschema.NewStringLoader(string(data))

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		// If schema file doesn't exist, skip validation (backwards compatibility)
		// Fall through to unmarshaling with strict mode
	} else if !result.Valid() {
		errMsg := "JSON Schema validation failed:\n"
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Step 2: Unmarshal with strict mode (DisallowUnknownFields for extra protection)
	var schema database.Schema
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	return &schema, nil
}

// ValidateJSONSchema validates a JSON file without loading it
func ValidateJSONSchema(path string) error {
	_, err := LoadJSONSchema(path)
	return err
}
