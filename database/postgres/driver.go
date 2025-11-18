package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
)

// Driver implements database.Driver for PostgreSQL
type Driver struct {
	*Introspector
	*Generator
}

// NewDriver creates a new PostgreSQL driver
func NewDriver() *Driver {
	return &Driver{
		Introspector: NewIntrospector(),
		Generator:    NewGenerator(),
	}
}

// Name returns the database driver name
func (d *Driver) Name() string {
	return "postgres"
}

// SupportsFeature checks if PostgreSQL supports a specific feature
func (d *Driver) SupportsFeature(feature string) bool {
	switch feature {
	case "CASCADE":
		return true
	case "ALTER_COLUMN_TYPE":
		return true
	case "ALTER_COLUMN_NULLABLE":
		return true
	case "ALTER_COLUMN_DEFAULT":
		return true
	case "FOREIGN_KEYS":
		return true
	case "information_schema":
		return true
	default:
		return false
	}
}

// Ensure Driver implements database.Driver
var _ database.Driver = (*Driver)(nil)

// Ensure Introspector implements database.Introspector
var _ database.Introspector = (*Introspector)(nil)

// Ensure Generator implements database.SQLGenerator
var _ database.SQLGenerator = (*Generator)(nil)

// Helper methods to satisfy the interface

func (d *Driver) IntrospectSchema(ctx context.Context, db *sql.DB) (*database.Schema, error) {
	return d.Introspector.IntrospectSchema(ctx, db)
}

func (d *Driver) IntrospectSchemas(ctx context.Context, db *sql.DB, schemas []string) (*database.Schema, error) {
	return d.Introspector.IntrospectSchemas(ctx, db, schemas)
}

func (d *Driver) GetTables(ctx context.Context, db *sql.DB) ([]string, error) {
	return d.Introspector.GetTables(ctx, db)
}

func (d *Driver) GetColumns(ctx context.Context, db *sql.DB, tableName string) ([]database.Column, error) {
	return d.Introspector.GetColumns(ctx, db, tableName)
}

func (d *Driver) GetIndexes(ctx context.Context, db *sql.DB, tableName string) ([]database.Index, error) {
	return d.Introspector.GetIndexes(ctx, db, tableName)
}

func (d *Driver) GetForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]database.ForeignKey, error) {
	return d.Introspector.GetForeignKeys(ctx, db, tableName)
}

func (d *Driver) CreateTable(table database.Table) (string, string) {
	return d.Generator.CreateTable(table)
}

func (d *Driver) DropTable(table database.Table) (string, string) {
	return d.Generator.DropTable(table)
}

func (d *Driver) AddColumn(tableName string, col database.Column) (string, string) {
	return d.Generator.AddColumn(tableName, col)
}

func (d *Driver) DropColumn(tableName string, col database.Column) (string, string) {
	return d.Generator.DropColumn(tableName, col)
}

func (d *Driver) ModifyColumn(tableName string, diff database.ColumnDiff) []database.PlanStep {
	return d.Generator.ModifyColumn(tableName, diff)
}

func (d *Driver) AddIndex(tableName string, idx database.Index) (string, string) {
	return d.Generator.AddIndex(tableName, idx)
}

func (d *Driver) DropIndex(tableName string, idx database.Index) (string, string) {
	return d.Generator.DropIndex(tableName, idx)
}

func (d *Driver) AddForeignKey(tableName string, fk database.ForeignKey) (string, string) {
	return d.Generator.AddForeignKey(tableName, fk)
}

func (d *Driver) DropForeignKey(tableName string, fk database.ForeignKey) (string, string) {
	return d.Generator.DropForeignKey(tableName, fk)
}

func (d *Driver) FormatColumnDefinition(col database.Column) string {
	return d.Generator.FormatColumnDefinition(col)
}

func (d *Driver) ParameterPlaceholder(position int) string {
	return d.Generator.ParameterPlaceholder(position)
}

// SupportsSchemas returns true for PostgreSQL (supports schema namespaces)
func (d *Driver) SupportsSchemas() bool {
	return true
}

// CreateSchema creates a schema in PostgreSQL
func (d *Driver) CreateSchema(ctx context.Context, db *sql.DB, schemaName string) error {
	// Use quoted identifier to prevent SQL injection
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", quoteIdentifier(schemaName))
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema %q: %w", schemaName, err)
	}
	return nil
}

// SetSchema sets the current search path to the specified schema
func (d *Driver) SetSchema(ctx context.Context, db *sql.DB, schemaName string) error {
	// Use quoted identifier to prevent SQL injection
	query := fmt.Sprintf("SET search_path TO %s", quoteIdentifier(schemaName))
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set search path to %q: %w", schemaName, err)
	}
	return nil
}

// DropSchema drops a schema from PostgreSQL
func (d *Driver) DropSchema(ctx context.Context, db *sql.DB, schemaName string, cascade bool) error {
	// Use quoted identifier to prevent SQL injection
	cascadeStr := ""
	if cascade {
		cascadeStr = " CASCADE"
	}
	query := fmt.Sprintf("DROP SCHEMA IF EXISTS %s%s", quoteIdentifier(schemaName), cascadeStr)
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop schema %q: %w", schemaName, err)
	}
	return nil
}

// ListSchemas returns all schema names in the database
func (d *Driver) ListSchemas(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schema_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schemas []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, fmt.Errorf("failed to scan schema name: %w", err)
		}
		schemas = append(schemas, schemaName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating schemas: %w", err)
	}

	return schemas, nil
}

// quoteIdentifier quotes a PostgreSQL identifier to prevent SQL injection
func quoteIdentifier(identifier string) string {
	// Replace any double quotes with escaped double quotes
	escaped := strings.ReplaceAll(identifier, `"`, `""`)
	return fmt.Sprintf(`"%s"`, escaped)
}
