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
	schema := &database.Schema{}

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
	defer rows.Close()

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
	defer rows.Close()

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

		col.Nullable = notNull == 0
		col.IsPrimaryKey = pk > 0
		if defaultVal.Valid {
			col.Default = &defaultVal.String
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
	defer rows.Close()

	var indexes []database.Index
	for rows.Next() {
		var seq int
		var idx database.Index
		var origin string
		var partial int
		var unique int

		// PRAGMA index_list returns: seq, name, unique, origin, partial
		if err := rows.Scan(&seq, &idx.Name, &unique, &origin, &partial); err != nil {
			return nil, err
		}

		idx.Unique = unique == 1

		// Get columns for this index
		indexInfoQuery := fmt.Sprintf("PRAGMA index_info(%s)", idx.Name)
		indexRows, err := db.QueryContext(ctx, indexInfoQuery)
		if err != nil {
			return nil, err
		}

		for indexRows.Next() {
			var seqno, cid int
			var name sql.NullString

			// PRAGMA index_info returns: seqno, cid, name
			if err := indexRows.Scan(&seqno, &cid, &name); err != nil {
				indexRows.Close()
				return nil, err
			}

			if name.Valid {
				idx.Columns = append(idx.Columns, name.String)
			}
		}
		indexRows.Close()

		// Skip auto-created indexes (like for primary keys)
		if origin != "c" && !strings.HasPrefix(idx.Name, "sqlite_autoindex") {
			indexes = append(indexes, idx)
		}
	}

	return indexes, nil
}

// GetForeignKeys returns all foreign keys for a given SQLite table
func (i *Introspector) GetForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]database.ForeignKey, error) {
	// SQLite uses PRAGMA foreign_key_list
	query := fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
