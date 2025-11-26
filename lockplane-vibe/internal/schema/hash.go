package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lockplane/lockplane/database"
)

// ComputeSchemaHash generates a deterministic hash of a schema.
// The hash represents the complete state of the schema including all tables,
// columns, indexes, and foreign keys. Any change to the schema will produce
// a different hash.
func ComputeSchemaHash(schema *database.Schema) (string, error) {
	// Create a normalized representation for hashing
	normalized, err := normalizeSchema(schema)
	if err != nil {
		return "", fmt.Errorf("failed to normalize schema: %w", err)
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:]), nil
}

// normalizeSchema creates a canonical string representation of the schema
func normalizeSchema(schema *database.Schema) (string, error) {
	// Handle nil schema - treat it the same as empty schema
	if schema == nil {
		return `{"tables":[]}`, nil
	}

	// Sort tables by name for consistent ordering
	sortedTables := make([]database.Table, len(schema.Tables))
	copy(sortedTables, schema.Tables)
	sort.Slice(sortedTables, func(i, j int) bool {
		return sortedTables[i].Name < sortedTables[j].Name
	})

	// Create a map of normalized tables
	normalized := make(map[string]interface{})
	tables := make([]map[string]interface{}, 0, len(sortedTables))

	for _, table := range sortedTables {
		tableMap := map[string]interface{}{
			"name":    table.Name,
			"columns": normalizeColumns(table.Columns),
		}

		if len(table.Indexes) > 0 {
			tableMap["indexes"] = normalizeIndexes(table.Indexes)
		}

		if len(table.ForeignKeys) > 0 {
			tableMap["foreign_keys"] = normalizeForeignKeys(table.ForeignKeys)
		}

		tables = append(tables, tableMap)
	}

	normalized["tables"] = tables

	// Marshal to JSON for consistent string representation
	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to marshal normalized schema: %w", err)
	}

	return string(jsonBytes), nil
}

// normalizeColumns creates a canonical representation of columns
func normalizeColumns(columns []database.Column) []map[string]interface{} {
	// Sort columns by name
	sorted := make([]database.Column, len(columns))
	copy(sorted, columns)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	result := make([]map[string]interface{}, len(sorted))
	for i, col := range sorted {
		colMap := map[string]interface{}{
			"name":           col.Name,
			"type":           strings.ToLower(col.Type),
			"nullable":       col.Nullable,
			"is_primary_key": col.IsPrimaryKey,
		}

		if col.Default != nil {
			colMap["default"] = *col.Default
		}

		result[i] = colMap
	}

	return result
}

// normalizeIndexes creates a canonical representation of indexes
func normalizeIndexes(indexes []database.Index) []map[string]interface{} {
	// Sort indexes by name
	sorted := make([]database.Index, len(indexes))
	copy(sorted, indexes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	result := make([]map[string]interface{}, len(sorted))
	for i, idx := range sorted {
		result[i] = map[string]interface{}{
			"name":    idx.Name,
			"columns": idx.Columns,
			"unique":  idx.Unique,
		}
	}

	return result
}

// normalizeForeignKeys creates a canonical representation of foreign keys
func normalizeForeignKeys(fks []database.ForeignKey) []map[string]interface{} {
	// Sort foreign keys by name
	sorted := make([]database.ForeignKey, len(fks))
	copy(sorted, fks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	result := make([]map[string]interface{}, len(sorted))
	for i, fk := range sorted {
		fkMap := map[string]interface{}{
			"name":               fk.Name,
			"columns":            fk.Columns,
			"referenced_table":   fk.ReferencedTable,
			"referenced_columns": fk.ReferencedColumns,
		}

		if fk.OnDelete != nil {
			fkMap["on_delete"] = *fk.OnDelete
		}

		if fk.OnUpdate != nil {
			fkMap["on_update"] = *fk.OnUpdate
		}

		result[i] = fkMap
	}

	return result
}
