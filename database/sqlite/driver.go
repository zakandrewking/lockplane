package sqlite

import (
	"context"
	"database/sql"

	"github.com/lockplane/lockplane/database"
)

// Driver implements database.Driver for SQLite
type Driver struct {
	*Introspector
	*Generator
}

// NewDriver creates a new SQLite driver
func NewDriver() *Driver {
	return &Driver{
		Introspector: NewIntrospector(),
		Generator:    NewGenerator(),
	}
}

// Name returns the database driver name
func (d *Driver) Name() string {
	return "sqlite"
}

// SupportsFeature checks if SQLite supports a specific feature
func (d *Driver) SupportsFeature(feature string) bool {
	switch feature {
	case "CASCADE":
		return false // SQLite doesn't support CASCADE on DROP TABLE
	case "ALTER_COLUMN_TYPE":
		return false // Would require table recreation
	case "ALTER_COLUMN_NULLABLE":
		return false // Would require table recreation
	case "ALTER_COLUMN_DEFAULT":
		return false // Would require table recreation
	case "ALTER_ADD_FOREIGN_KEY":
		return false // Foreign keys must be defined at table creation
	case "FOREIGN_KEYS":
		return true // Supports foreign keys at table creation
	case "DROP_COLUMN":
		return true // SQLite 3.35.0+
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

func (d *Driver) RecreateTableWithForeignKey(table database.Table, fk database.ForeignKey) []database.PlanStep {
	return d.Generator.RecreateTableWithForeignKey(table, fk)
}

func (d *Driver) RecreateTableWithoutForeignKey(table database.Table, fkName string) []database.PlanStep {
	return d.Generator.RecreateTableWithoutForeignKey(table, fkName)
}
