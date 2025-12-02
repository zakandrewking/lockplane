package driver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/driver/postgres"
	"github.com/lockplane/lockplane/internal/schema"
)

type Generator interface {
	// Generate migration from schema diff
	GenerateMigration(diff *schema.SchemaDiff) string

	// CreateTable generates SQL to create a table
	CreateTable(table database.Table) string

	// DropTable generates SQL to drop a table
	DropTable(table database.Table) string

	// AddColumn generates SQL to add a column to a table
	AddColumn(tableName string, col database.Column) string

	// DropColumn generates SQL to drop a column from a table
	DropColumn(tableName string, col database.Column) string

	// ModifyColumn generates SQL to modify a column (type, nullability, default)
	// Returns multiple steps if needed (e.g., SQLite table recreation)
	ModifyColumn(tableName string, diff schema.ColumnDiff) string

	// FormatColumnDefinition formats a column definition for CREATE TABLE
	FormatColumnDefinition(col database.Column) string
}

// Driver represents a database driver with introspection and SQL generation
type Driver interface {
	Generator

	// Name returns the database driver name
	Name() string

	// TestConnection attempts to connect to the database
	OpenConnection(cfg database.ConnectionConfig) (*sql.DB, error)

	// IntrospectSchema reads the entire database schema
	IntrospectSchema(ctx context.Context, db *sql.DB, schemaName string) (*database.Schema, error)

	// Shortcut for Generator.GenerateMigration
	GenerateMigration(diff *schema.SchemaDiff) string

	// Apply migration to the database
	ApplyMigration(ctx context.Context, db *sql.DB, migration string) error
}

// NewDriver creates a new database driver based on the driver name.
func NewDriver(databaseType database.DatabaseType) (Driver, error) {
	switch databaseType {
	case database.DatabaseTypePostgres:
		return postgres.NewDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", databaseType)
	}
}
