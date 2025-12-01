package postgres

import (
	"context"
	"database/sql"
	"fmt"
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
		// TODO defer necessary?
		defer func() { _ = db.Close() }()
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
		Tables: tables,
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
	// TODO make?
	var tables = make([]database.Table, 0)
	for _, tableName := range tableNames {
		table := database.Table{
			Name:   tableName,
			Schema: schemaName,
		}

		// columns, err := i.GetColumnsInSchema(ctx, db, schemaName, tableName)
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to get columns for table %s.%s: %w", schemaName, tableName, err)
		// }
		// table.Columns = columns

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

		tables = append(tables, table)
	}

	return tables, nil
}
