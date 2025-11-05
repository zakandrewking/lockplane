package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/lockplane/lockplane/database"
)

// ComputeSchemaHash generates a deterministic hash of a schema.
// The hash represents the complete state of the schema including all tables,
// columns, indexes, and foreign keys. Any change to the schema will produce
// a different hash.
func ComputeSchemaHash(schema *Schema) (string, error) {
	if schema == nil {
		// Empty schema has a special hash
		return computeHash(map[string]interface{}{"tables": []interface{}{}}), nil
	}

	// Create a canonical representation of the schema
	// Sort everything to ensure deterministic output
	canonical := canonicalizeSchema(schema)

	// Convert to JSON for hashing
	jsonBytes, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}

	return computeHash(string(jsonBytes)), nil
}

// canonicalizeSchema creates a sorted, deterministic representation of a schema
func canonicalizeSchema(schema *Schema) map[string]interface{} {
	tables := make([]interface{}, 0, len(schema.Tables))

	// Sort tables by name
	sortedTables := make([]database.Table, len(schema.Tables))
	copy(sortedTables, schema.Tables)
	sort.Slice(sortedTables, func(i, j int) bool {
		return sortedTables[i].Name < sortedTables[j].Name
	})

	for _, table := range sortedTables {
		tableMap := map[string]interface{}{
			"name":    table.Name,
			"columns": canonicalizeColumns(table.Columns),
		}

		if len(table.Indexes) > 0 {
			tableMap["indexes"] = canonicalizeIndexes(table.Indexes)
		}

		if len(table.ForeignKeys) > 0 {
			tableMap["foreign_keys"] = canonicalizeForeignKeys(table.ForeignKeys)
		}

		tables = append(tables, tableMap)
	}

	return map[string]interface{}{
		"tables": tables,
	}
}

func canonicalizeColumns(columns []database.Column) []interface{} {
	result := make([]interface{}, 0, len(columns))

	// Sort columns by name
	sortedCols := make([]database.Column, len(columns))
	copy(sortedCols, columns)
	sort.Slice(sortedCols, func(i, j int) bool {
		return sortedCols[i].Name < sortedCols[j].Name
	})

	for _, col := range sortedCols {
		colMap := map[string]interface{}{
			"name":           col.Name,
			"type":           col.LogicalType(), // Use logical type for cross-dialect compatibility
			"nullable":       col.Nullable,
			"is_primary_key": col.IsPrimaryKey,
		}

		if col.Default != nil {
			colMap["default"] = *col.Default
		}

		result = append(result, colMap)
	}

	return result
}

func canonicalizeIndexes(indexes []database.Index) []interface{} {
	result := make([]interface{}, 0, len(indexes))

	// Sort indexes by name
	sortedIndexes := make([]database.Index, len(indexes))
	copy(sortedIndexes, indexes)
	sort.Slice(sortedIndexes, func(i, j int) bool {
		return sortedIndexes[i].Name < sortedIndexes[j].Name
	})

	for _, idx := range sortedIndexes {
		idxMap := map[string]interface{}{
			"name":    idx.Name,
			"columns": idx.Columns, // Already sorted in schema
			"unique":  idx.Unique,
		}

		result = append(result, idxMap)
	}

	return result
}

func canonicalizeForeignKeys(fks []database.ForeignKey) []interface{} {
	result := make([]interface{}, 0, len(fks))

	// Sort foreign keys by first column name
	sortedFKs := make([]database.ForeignKey, len(fks))
	copy(sortedFKs, fks)
	sort.Slice(sortedFKs, func(i, j int) bool {
		if len(sortedFKs[i].Columns) > 0 && len(sortedFKs[j].Columns) > 0 {
			return sortedFKs[i].Columns[0] < sortedFKs[j].Columns[0]
		}
		return sortedFKs[i].Name < sortedFKs[j].Name
	})

	for _, fk := range sortedFKs {
		fkMap := map[string]interface{}{
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

		result = append(result, fkMap)
	}

	return result
}

func computeHash(data interface{}) string {
	var bytes []byte

	switch v := data.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		// For other types, convert to JSON first
		jsonBytes, _ := json.Marshal(v)
		bytes = jsonBytes
	}

	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}
