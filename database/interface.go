package database

import (
	"context"
	"database/sql"
	"strings"
)

// Schema represents a database schema
type Schema struct {
	Tables  []Table `json:"tables"`
	Dialect Dialect `json:"dialect,omitempty"`
}

// Table represents a database table
type Table struct {
	Name        string       `json:"name"`
	Schema      string       `json:"schema,omitempty"` // Schema name (e.g., "public", "storage")
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
	RLSEnabled  bool         `json:"rls_enabled,omitempty"`
	Policies    []Policy     `json:"policies,omitempty"` // Row Level Security policies
}

// Column represents a table column
type Column struct {
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	Nullable        bool             `json:"nullable"`
	Default         *string          `json:"default,omitempty"`
	IsPrimaryKey    bool             `json:"is_primary_key"`
	TypeMetadata    *TypeMetadata    `json:"type_metadata,omitempty"`
	DefaultMetadata *DefaultMetadata `json:"default_metadata,omitempty"`
}

// Index represents a table index
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

// ForeignKey represents a foreign key constraint
type ForeignKey struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedTable   string   `json:"referenced_table"`
	ReferencedColumns []string `json:"referenced_columns"`
	OnDelete          *string  `json:"on_delete,omitempty"`
	OnUpdate          *string  `json:"on_update,omitempty"`
}

// Policy represents a Row Level Security (RLS) policy
type Policy struct {
	Name       string   `json:"name"`
	Command    string   `json:"command"`              // SELECT, INSERT, UPDATE, DELETE, ALL
	Permissive bool     `json:"permissive"`           // true = PERMISSIVE (default), false = RESTRICTIVE
	Roles      []string `json:"roles"`                // Roles this policy applies to (empty = all roles)
	Using      *string  `json:"using,omitempty"`      // USING clause (for SELECT, UPDATE, DELETE)
	WithCheck  *string  `json:"with_check,omitempty"` // WITH CHECK clause (for INSERT, UPDATE)
}

// Dialect represents the database dialect associated with a schema
type Dialect string

const (
	// DialectUnknown indicates we could not determine the dialect
	DialectUnknown Dialect = ""
	// DialectPostgres represents PostgreSQL
	DialectPostgres Dialect = "postgres"
	// DialectSQLite represents SQLite (and libsql/Turso)
	DialectSQLite Dialect = "sqlite"
)

// TypeMetadata captures both logical and raw type representations.
type TypeMetadata struct {
	Logical string  `json:"logical,omitempty"`
	Raw     string  `json:"raw,omitempty"`
	Dialect Dialect `json:"dialect,omitempty"`
}

// DefaultMetadata captures raw default expressions with dialect information.
type DefaultMetadata struct {
	Raw     string  `json:"raw,omitempty"`
	Dialect Dialect `json:"dialect,omitempty"`
	Kind    string  `json:"kind,omitempty"`
}

// LogicalType returns the normalized type name used for comparisons.
func (c Column) LogicalType() string {
	if c.TypeMetadata != nil && c.TypeMetadata.Logical != "" {
		return strings.ToLower(c.TypeMetadata.Logical)
	}
	return strings.ToLower(c.Type)
}

// Introspector defines the interface for database schema introspection
type Introspector interface {
	// IntrospectSchema reads the entire database schema
	IntrospectSchema(ctx context.Context, db *sql.DB) (*Schema, error)

	// GetTables returns all table names in the database
	GetTables(ctx context.Context, db *sql.DB) ([]string, error)

	// GetColumns returns all columns for a given table
	GetColumns(ctx context.Context, db *sql.DB, tableName string) ([]Column, error)

	// GetIndexes returns all indexes for a given table
	GetIndexes(ctx context.Context, db *sql.DB, tableName string) ([]Index, error)

	// GetForeignKeys returns all foreign keys for a given table
	GetForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]ForeignKey, error)
}

// ColumnDiff represents changes to a column
type ColumnDiff struct {
	ColumnName string
	Old        Column
	New        Column
	Changes    []string // e.g., ["type", "nullable", "default"]
}

// PlanStep represents a single logical migration operation in a migration plan
// that may consist of multiple SQL statements executed atomically
type PlanStep struct {
	Description string   `json:"description"`
	SQL         []string `json:"sql"` // Array of SQL statements to execute in order
}

// SQLGenerator defines the interface for generating database-specific SQL
type SQLGenerator interface {
	// CreateTable generates SQL to create a table
	CreateTable(table Table) (sql string, description string)

	// DropTable generates SQL to drop a table
	DropTable(table Table) (sql string, description string)

	// AddColumn generates SQL to add a column to a table
	AddColumn(tableName string, col Column) (sql string, description string)

	// DropColumn generates SQL to drop a column from a table
	DropColumn(tableName string, col Column) (sql string, description string)

	// ModifyColumn generates SQL to modify a column (type, nullability, default)
	// Returns multiple steps if needed (e.g., SQLite table recreation)
	ModifyColumn(tableName string, diff ColumnDiff) []PlanStep

	// AddIndex generates SQL to add an index
	AddIndex(tableName string, idx Index) (sql string, description string)

	// DropIndex generates SQL to drop an index
	DropIndex(tableName string, idx Index) (sql string, description string)

	// AddForeignKey generates SQL to add a foreign key constraint
	AddForeignKey(tableName string, fk ForeignKey) (sql string, description string)

	// DropForeignKey generates SQL to drop a foreign key constraint
	DropForeignKey(tableName string, fk ForeignKey) (sql string, description string)

	// FormatColumnDefinition formats a column definition for CREATE TABLE
	FormatColumnDefinition(col Column) string

	// ParameterPlaceholder returns the parameter placeholder for this database
	// PostgreSQL: $1, $2, etc.
	// SQLite: ?, ?, etc.
	ParameterPlaceholder(position int) string
}

// Driver represents a database driver with introspection and SQL generation
type Driver interface {
	Introspector
	SQLGenerator

	// Name returns the database driver name (e.g., "postgres", "sqlite")
	Name() string

	// SupportsFeature checks if the database supports a specific feature
	SupportsFeature(feature string) bool

	// Schema support (PostgreSQL only)
	// SupportsSchemas returns true if the database supports schema namespaces
	SupportsSchemas() bool

	// IntrospectSchemas introspects multiple schemas (PostgreSQL only)
	// If schemas is nil or empty, behaves like IntrospectSchema (uses current_schema())
	// Returns a combined Schema with tables from all specified schemas
	IntrospectSchemas(ctx context.Context, db *sql.DB, schemas []string) (*Schema, error)

	// CreateSchema creates a schema in the database (no-op if not supported)
	CreateSchema(ctx context.Context, db *sql.DB, schemaName string) error

	// SetSchema sets the current schema/search path (no-op if not supported)
	SetSchema(ctx context.Context, db *sql.DB, schemaName string) error

	// DropSchema drops a schema from the database (no-op if not supported)
	DropSchema(ctx context.Context, db *sql.DB, schemaName string, cascade bool) error

	// ListSchemas returns all schema names in the database (empty if not supported)
	ListSchemas(ctx context.Context, db *sql.DB) ([]string, error)
}
