package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

type Schema struct {
	Tables []Table `json:"tables"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Indexes []Index  `json:"indexes"`
}

type Column struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default,omitempty"`
	IsPrimaryKey bool    `json:"is_primary_key"`
}

type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

func main() {
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	schema, err := introspectSchema(ctx, db)
	if err != nil {
		log.Fatalf("Failed to introspect schema: %v", err)
	}

	output, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal schema: %v", err)
	}

	fmt.Println(string(output))
}

func introspectSchema(ctx context.Context, db *sql.DB) (*Schema, error) {
	schema := &Schema{}

	// Get all tables in public schema
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
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	// For each table, get columns and indexes
	for _, tableName := range tableNames {
		table := Table{Name: tableName}

		// Get columns
		columns, err := getColumns(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}
		table.Columns = columns

		// Get indexes
		indexes, err := getIndexes(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for table %s: %w", tableName, err)
		}
		table.Indexes = indexes

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func getColumns(ctx context.Context, db *sql.DB, tableName string) ([]Column, error) {
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
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var nullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &col.IsPrimaryKey); err != nil {
			return nil, err
		}

		col.Nullable = nullable == "YES"
		if defaultVal.Valid {
			col.Default = &defaultVal.String
		}

		columns = append(columns, col)
	}

	return columns, nil
}

func getIndexes(ctx context.Context, db *sql.DB, tableName string) ([]Index, error) {
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
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var idx Index
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
