package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
)

// Introspector implements database.Introspector for SQLite
type Introspector struct{}

// NewIntrospector creates a new SQLite introspector
func NewIntrospector() *Introspector {
	return &Introspector{}
}

// IntrospectSchema reads the entire SQLite database schema
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

	schema.Dialect = database.DialectSQLite
	return schema, nil
}

// GetTables returns all table names in the SQLite database
func (i *Introspector) GetTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
            SELECT name
            FROM sqlite_master
            WHERE type = 'table'
            AND name NOT LIKE 'sqlite_%'
            ORDER BY name
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

// GetColumns returns all columns for a given SQLite table
func (i *Introspector) GetColumns(ctx context.Context, db *sql.DB, tableName string) ([]database.Column, error) {
	// SQLite uses PRAGMA table_info
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []database.Column
	for rows.Next() {
		var cid int
		var col database.Column
		var notNull int
		var defaultVal sql.NullString
		var pk int

		// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
		if err := rows.Scan(&cid, &col.Name, &col.Type, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}

		col.Type = strings.TrimSpace(col.Type)
		logical := strings.ToLower(col.Type)
		col.TypeMetadata = &database.TypeMetadata{
			Logical: logical,
			Raw:     col.Type,
			Dialect: database.DialectSQLite,
		}

		col.Nullable = notNull == 0
		col.IsPrimaryKey = pk > 0
		if defaultVal.Valid {
			col.Default = &defaultVal.String
			col.DefaultMetadata = &database.DefaultMetadata{
				Raw:     defaultVal.String,
				Dialect: database.DialectSQLite,
			}
		} else {
			col.DefaultMetadata = nil
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// GetIndexes returns all indexes for a given SQLite table
func (i *Introspector) GetIndexes(ctx context.Context, db *sql.DB, tableName string) ([]database.Index, error) {
	// Get index list
	query := fmt.Sprintf("PRAGMA index_list(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	type rawIndex struct {
		index  database.Index
		origin string
	}

	var rawIndexes []rawIndex
	for rows.Next() {
		var seq int
		var origin string
		var partial int
		var unique int
		var name string

		// PRAGMA index_list returns: seq, name, unique, origin, partial
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}

		_ = seq
		_ = partial

		rawIndexes = append(rawIndexes, rawIndex{
			index: database.Index{
				Name:   name,
				Unique: unique == 1,
			},
			origin: origin,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = rows.Close()

	var indexes []database.Index
	for _, raw := range rawIndexes {
		indexInfoQuery := fmt.Sprintf("PRAGMA index_info(%s)", quoteSQLiteString(raw.index.Name))
		indexRows, indexErr := db.QueryContext(ctx, indexInfoQuery)
		if indexErr != nil {
			return nil, fmt.Errorf("failed to query index_info for %s: %w", raw.index.Name, indexErr)
		}

		for indexRows.Next() {
			var seqno, cid int
			var name sql.NullString

			// PRAGMA index_info returns: seqno, cid, name
			if err := indexRows.Scan(&seqno, &cid, &name); err != nil {
				_ = indexRows.Close()
				return nil, fmt.Errorf("failed to scan index_info for %s: %w", raw.index.Name, err)
			}

			if name.Valid {
				raw.index.Columns = append(raw.index.Columns, name.String)
			}
		}
		if err := indexRows.Err(); err != nil {
			_ = indexRows.Close()
			return nil, fmt.Errorf("error iterating index_info for %s: %w", raw.index.Name, err)
		}
		_ = indexRows.Close()

		if raw.origin == "c" {
			indexes = append(indexes, raw.index)
		}
	}

	return indexes, nil
}

func quoteSQLiteString(value string) string {
	escaped := strings.ReplaceAll(value, "'", "''")
	return fmt.Sprintf("'%s'", escaped)
}

// GetForeignKeys returns all foreign keys for a given SQLite table
func (i *Introspector) GetForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]database.ForeignKey, error) {
	// SQLite uses PRAGMA foreign_key_list
	query := fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Group by id (foreign key constraint ID)
	fkMap := make(map[int]*database.ForeignKey)
	var fkIds []int

	for rows.Next() {
		var id, seq int
		var table, from, to string
		var onUpdate, onDelete, match string

		// PRAGMA foreign_key_list returns: id, seq, table, from, to, on_update, on_delete, match
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}

		if _, exists := fkMap[id]; !exists {
			fk := &database.ForeignKey{
				Name:              fmt.Sprintf("fk_%s_%d", tableName, id),
				Columns:           []string{},
				ReferencedTable:   table,
				ReferencedColumns: []string{},
			}

			if onUpdate != "NO ACTION" {
				fk.OnUpdate = &onUpdate
			}
			if onDelete != "NO ACTION" {
				fk.OnDelete = &onDelete
			}

			fkMap[id] = fk
			fkIds = append(fkIds, id)
		}

		fkMap[id].Columns = append(fkMap[id].Columns, from)
		fkMap[id].ReferencedColumns = append(fkMap[id].ReferencedColumns, to)
	}

	// Convert map to slice in consistent order
	var foreignKeys []database.ForeignKey
	for _, id := range fkIds {
		foreignKeys = append(foreignKeys, *fkMap[id])
	}

	return foreignKeys, nil
}
