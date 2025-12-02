package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/lockplane/lockplane/internal/database"
)

// Driver implements database.Driver for PostgreSQL
type Driver struct {
}

// NewDriver creates a new PostgreSQL driver
func NewDriver() *Driver {
	return &Driver{}
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

		// // Get RLS status
		// rlsEnabled, err := i.GetRLSEnabledInSchema(ctx, db, schemaName, tableName)
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to get RLS status for table %s.%s: %w", schemaName, tableName, err)
		// }
		// table.RLSEnabled = rlsEnabled

		// // Get RLS policies if RLS is enabled
		// if rlsEnabled {
		// 	policies, err := i.GetPoliciesInSchema(ctx, db, schemaName, tableName)
		// 	if err != nil {
		// 		return nil, fmt.Errorf("failed to get policies for table %s.%s: %w", schemaName, tableName, err)
		// 	}
		// 	table.Policies = policies
		// }

		table := database.Table{
			Name:    tableName,
			Schema:  schemaName,
			Columns: columns,
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

		columns = append(columns, col)
	}

	return columns, nil
}
