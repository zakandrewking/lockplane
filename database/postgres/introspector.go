package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
)

// Introspector implements database.Introspector for PostgreSQL
type Introspector struct{}

// NewIntrospector creates a new PostgreSQL introspector
func NewIntrospector() *Introspector {
	return &Introspector{}
}

// IntrospectSchema reads the entire PostgreSQL database schema
func (i *Introspector) IntrospectSchema(ctx context.Context, db *sql.DB) (*database.Schema, error) {
	schema := &database.Schema{
		Tables: make([]database.Table, 0),
	}

	tables, err := i.GetTables(ctx, db)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		table := database.Table{Name: tableName}

		columns, err := i.GetColumns(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}
		table.Columns = columns

		indexes, err := i.GetIndexes(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for table %s: %w", tableName, err)
		}
		table.Indexes = indexes

		foreignKeys, err := i.GetForeignKeys(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get foreign keys for table %s: %w", tableName, err)
		}
		table.ForeignKeys = foreignKeys

		schema.Tables = append(schema.Tables, table)
	}

	schema.Dialect = database.DialectPostgres
	return schema, nil
}

// GetTables returns all table names in the PostgreSQL database
func (i *Introspector) GetTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
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

	return tableNames, nil
}

// GetColumns returns all columns for a given PostgreSQL table
func (i *Introspector) GetColumns(ctx context.Context, db *sql.DB, tableName string) ([]database.Column, error) {
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
				 WHERE tc.table_name = c.table_name
				   AND tc.constraint_type = 'PRIMARY KEY'
				   AND kcu.column_name = c.column_name),
				false
			) as is_primary_key
		FROM information_schema.columns c
		WHERE c.table_schema = 'public'
		  AND c.table_name = $1
		ORDER BY c.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
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
		col.TypeMetadata = &database.TypeMetadata{
			Logical: strings.ToLower(col.Type),
			Raw:     col.Type,
			Dialect: database.DialectPostgres,
		}

		col.Nullable = nullable == "YES"
		if defaultVal.Valid {
			col.Default = &defaultVal.String
			col.DefaultMetadata = &database.DefaultMetadata{
				Raw:     defaultVal.String,
				Dialect: database.DialectPostgres,
			}
		} else {
			col.DefaultMetadata = nil
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// GetIndexes returns all indexes for a given PostgreSQL table
func (i *Introspector) GetIndexes(ctx context.Context, db *sql.DB, tableName string) ([]database.Index, error) {
	query := `
		SELECT
			i.indexname,
			i.indexdef,
			ix.indisunique
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.tablename
		JOIN pg_index ix ON ix.indexrelid = (
			SELECT oid FROM pg_class WHERE relname = i.indexname
		)
		WHERE i.schemaname = 'public'
		  AND i.tablename = $1
		ORDER BY i.indexname
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var indexes []database.Index
	for rows.Next() {
		var idx database.Index
		var indexDef string

		if err := rows.Scan(&idx.Name, &indexDef, &idx.Unique); err != nil {
			return nil, err
		}

		// TODO: Parse indexDef to extract column names properly
		// For now, just storing the index name and unique flag
		idx.Columns = []string{} // Simplified for MVP

		indexes = append(indexes, idx)
	}

	return indexes, nil
}

// GetForeignKeys returns all foreign keys for a given PostgreSQL table
func (i *Introspector) GetForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]database.ForeignKey, error) {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints AS rc
			ON rc.constraint_name = tc.constraint_name
			AND rc.constraint_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = 'public'
			AND tc.table_name = $1
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Group by constraint name to handle multi-column foreign keys
	fkMap := make(map[string]*database.ForeignKey)
	var fkNames []string

	for rows.Next() {
		var constraintName, columnName, foreignTableName, foreignColumnName string
		var updateRule, deleteRule string

		if err := rows.Scan(&constraintName, &columnName, &foreignTableName, &foreignColumnName, &updateRule, &deleteRule); err != nil {
			return nil, err
		}

		if _, exists := fkMap[constraintName]; !exists {
			fk := &database.ForeignKey{
				Name:              constraintName,
				Columns:           []string{},
				ReferencedTable:   foreignTableName,
				ReferencedColumns: []string{},
			}

			// Convert SQL standard actions to our format
			if updateRule != "NO ACTION" {
				fk.OnUpdate = &updateRule
			}
			if deleteRule != "NO ACTION" {
				fk.OnDelete = &deleteRule
			}

			fkMap[constraintName] = fk
			fkNames = append(fkNames, constraintName)
		}

		fkMap[constraintName].Columns = append(fkMap[constraintName].Columns, columnName)
		fkMap[constraintName].ReferencedColumns = append(fkMap[constraintName].ReferencedColumns, foreignColumnName)
	}

	// Convert map to slice in consistent order
	var foreignKeys []database.ForeignKey
	for _, name := range fkNames {
		foreignKeys = append(foreignKeys, *fkMap[name])
	}

	return foreignKeys, nil
}
