package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/schema"
)

// Driver implements database.Driver for PostgreSQL
type Driver struct {
	*Generator
}

// NewDriver creates a new PostgreSQL driver
func NewDriver() *Driver {
	return &Driver{
		Generator: NewGenerator(),
	}
}

// Name returns the database driver name
func (d *Driver) Name() string {
	return "postgres"
}

// Open a connection to the database, and run a ping to test it
func (d *Driver) OpenConnection(cfg database.ConnectionConfig) (*sql.DB, error) {
	// TODO enable ssl
	var finalUrl = cfg.PostgresUrl + "?sslmode=disable"
	db, err := sql.Open("postgres", finalUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Read the entire database schema
func (d *Driver) IntrospectSchema(ctx context.Context, db *sql.DB, schemaName string) (*database.Schema, error) {
	tables, err := GetTables(ctx, db, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables in schema %s: %w", schemaName, err)
	}

	schema := &database.Schema{
		Tables:  tables,
		Dialect: database.DialectPostgres,
	}

	return schema, nil
}

// return all table names in a specific PostgreSQL schema
func GetTables(ctx context.Context, db *sql.DB, schemaName string) ([]database.Table, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables in schema %s: %w", schemaName, err)
	}
	defer func() { _ = rows.Close() }()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	// Introspect each schema
	tables := []database.Table{}
	for _, tableName := range tableNames {
		columns, err := GetColumns(ctx, db, schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s.%s: %w", schemaName, tableName, err)
		}

		// indexes, err := i.GetIndexesInSchema(ctx, db, schemaName, tableName)
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to get indexes for table %s.%s: %w", schemaName, tableName, err)
		// }
		// table.Indexes = indexes

		// foreignKeys, err := i.GetForeignKeysInSchema(ctx, db, schemaName, tableName)
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to get foreign keys for table %s.%s: %w", schemaName, tableName, err)
		// }
		// table.ForeignKeys = foreignKeys

		// Get RLS status
		rlsEnabled, err := GetRLSEnabled(ctx, db, schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get RLS status for table %s.%s: %w", schemaName, tableName, err)
		}

		// // Get RLS policies if RLS is enabled
		// if rlsEnabled {
		// 	policies, err := GetPolicies(ctx, db, schemaName, tableName)
		// 	if err != nil {
		// 		return nil, fmt.Errorf("failed to get policies for table %s.%s: %w", schemaName, tableName, err)
		// 	}
		// 	table.Policies = policies
		// }

		table := database.Table{
			Name:       tableName,
			Schema:     schemaName,
			Columns:    columns,
			RLSEnabled: rlsEnabled,
		}

		tables = append(tables, table)
	}

	return tables, nil
}

// returns all columns for a given PostgreSQL table in a specific schema
func GetColumns(ctx context.Context, db *sql.DB, schemaName string, tableName string) ([]database.Column, error) {
	query := `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			COALESCE(
				(SELECT true
				 FROM information_schema.table_constraints tc
				 JOIN information_schema.key_column_usage kcu
				   ON tc.constraint_name = kcu.constraint_name
				   AND tc.table_schema = kcu.table_schema
				 WHERE tc.table_name = c.table_name
				   AND tc.table_schema = c.table_schema
				   AND tc.constraint_type = 'PRIMARY KEY'
				   AND kcu.column_name = c.column_name),
				false
			) as is_primary_key
		FROM information_schema.columns c
		WHERE c.table_schema = $1
		  AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []database.Column
	for rows.Next() {
		var col database.Column
		var nullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &col.IsPrimaryKey); err != nil {
			return nil, err
		}

		col.Type = strings.TrimSpace(col.Type)
		col.Nullable = nullable == "YES"

		if defaultVal.Valid {
			col.Default = &defaultVal.String
		}

		// Detect SERIAL types by checking if the column has an owned sequence
		// SERIAL creates an integer column with default nextval('table_column_seq'::regclass)
		// and the sequence is owned by the column
		if isSerialColumn(ctx, db, schemaName, tableName, col.Name, col.Type, col.Nullable, defaultVal) {
			switch col.Type {
			case "smallint":
				col.Type = "smallserial"
				col.Default = nil // SERIAL type implies the sequence, don't include default
			case "integer":
				col.Type = "serial"
				col.Default = nil
			case "bigint":
				col.Type = "bigserial"
				col.Default = nil
			}
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// isSerialColumn checks if a column with a nextval default is actually a SERIAL column
// by verifying that the sequence is owned by the column.
// This is stricter than just checking for "nextval" in the default value.
func isSerialColumn(ctx context.Context, db *sql.DB, schemaName, tableName, columnName, columnType string, nullable bool, defaultVal sql.NullString) bool {
	// SERIAL columns must be NOT NULL and have a default value
	if nullable || !defaultVal.Valid {
		return false
	}

	// Only check integer types
	if columnType != "smallint" && columnType != "integer" && columnType != "bigint" {
		return false
	}

	// Extract sequence name from default value using regex
	// Expected pattern: nextval('sequence_name'::regclass) or nextval('"schema"."sequence_name"'::regclass)
	re := regexp.MustCompile(`nextval\('([^']+)'::regclass\)`)
	matches := re.FindStringSubmatch(defaultVal.String)
	if len(matches) < 2 {
		return false
	}

	sequenceName := matches[1]

	// Strip schema qualifier if present (e.g., "public"."users_id_seq" -> users_id_seq)
	// Handle both quoted and unquoted identifiers
	if strings.Contains(sequenceName, ".") {
		parts := strings.Split(sequenceName, ".")
		sequenceName = parts[len(parts)-1]
	}
	// Remove quotes if present
	sequenceName = strings.Trim(sequenceName, "\"")

	// Query PostgreSQL system catalogs to verify the sequence is owned by this column
	// This joins pg_class (for sequence), pg_depend (for ownership), and pg_attribute (for column)
	query := `
		SELECT a.attname
		FROM pg_class s
		JOIN pg_depend d ON d.objid = s.oid AND d.classid = 'pg_class'::regclass
		JOIN pg_class t ON t.oid = d.refobjid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.refobjsubid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE s.relkind = 'S'
		  AND n.nspname = $1
		  AND t.relname = $2
		  AND s.relname = $3
	`

	var ownerColumn string
	err := db.QueryRowContext(ctx, query, schemaName, tableName, sequenceName).Scan(&ownerColumn)
	if err != nil {
		// If we can't find the sequence ownership, it's not a SERIAL column
		return false
	}

	// Check if the sequence is owned by this specific column
	return ownerColumn == columnName
}

// GetRLSEnabled checks if Row Level Security is enabled for a table
func GetRLSEnabled(ctx context.Context, db *sql.DB, schemaName string, tableName string) (bool, error) {
	query := `
		SELECT relrowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relname = $2
	`

	var rlsEnabled bool
	err := db.QueryRowContext(ctx, query, schemaName, tableName).Scan(&rlsEnabled)
	if err != nil {
		return false, fmt.Errorf("failed to query RLS status: %w", err)
	}

	return rlsEnabled, nil
}

func (d *Driver) GenerateMigration(diff *schema.SchemaDiff) string {
	return d.Generator.GenerateMigration(diff)
}

func (d *Driver) ApplyMigration(ctx context.Context, db *sql.DB, migration string) error {
	// Execute plan in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	_, err = tx.ExecContext(ctx, migration)
	if err != nil {
		// Rollback and preserve the original error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to execute migration: %w (rollback error: %v)", err, rbErr)
		}
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		// Cannot rollback after commit attempt - transaction is already complete
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
