package schema

import "github.com/lockplane/lockplane/database"

// SchemaDiff represents all differences between two schemas
type SchemaDiff struct {
	AddedTables    []database.Table `json:"added_tables,omitempty"`
	RemovedTables  []database.Table `json:"removed_tables,omitempty"`
	ModifiedTables []TableDiff      `json:"modified_tables,omitempty"`
}

// TableDiff represents changes to a single table
type TableDiff struct {
	TableName          string                `json:"table_name"`
	AddedColumns       []database.Column     `json:"added_columns,omitempty"`
	RemovedColumns     []database.Column     `json:"removed_columns,omitempty"`
	ModifiedColumns    []ColumnDiff          `json:"modified_columns,omitempty"`
	AddedIndexes       []database.Index      `json:"added_indexes,omitempty"`
	RemovedIndexes     []database.Index      `json:"removed_indexes,omitempty"`
	AddedForeignKeys   []database.ForeignKey `json:"added_foreign_keys,omitempty"`
	RemovedForeignKeys []database.ForeignKey `json:"removed_foreign_keys,omitempty"`
	RLSChanged         bool                  `json:"rls_changed,omitempty"`
	RLSEnabled         bool                  `json:"rls_enabled,omitempty"` // New value when RLSChanged is true
}

// ColumnDiff represents changes to a single column
type ColumnDiff struct {
	ColumnName string          `json:"column_name"`
	Old        database.Column `json:"old"`
	New        database.Column `json:"new"`
	Changes    []string        `json:"changes"` // e.g. ["type", "nullable", "default"]
}

// DiffSchemas compares two schemas and returns their differences
func DiffSchemas(current, desired *database.Schema) *SchemaDiff {
	diff := &SchemaDiff{}

	// Build maps for quick lookup
	currentTables := make(map[string]*database.Table)
	for i := range current.Tables {
		currentTables[current.Tables[i].Name] = &current.Tables[i]
	}

	desiredTables := make(map[string]*database.Table)
	for i := range desired.Tables {
		desiredTables[desired.Tables[i].Name] = &desired.Tables[i]
	}

	// Find added and modified tables
	for name, desiredTable := range desiredTables {
		currentTable, exists := currentTables[name]
		if !exists {
			// Table added
			diff.AddedTables = append(diff.AddedTables, *desiredTable)
		} else {
			// Table exists, check for modifications
			tableDiff := diffTables(currentTable, desiredTable)
			if !tableDiff.IsEmpty() {
				diff.ModifiedTables = append(diff.ModifiedTables, *tableDiff)
			}
		}
	}

	// Find removed tables
	for name, currentTable := range currentTables {
		if _, exists := desiredTables[name]; !exists {
			diff.RemovedTables = append(diff.RemovedTables, *currentTable)
		}
	}

	return diff
}

// diffTables compares two tables and returns their differences
func diffTables(current, desired *database.Table) *TableDiff {
	diff := &TableDiff{
		TableName: current.Name,
	}

	// Build maps for columns
	currentCols := make(map[string]*database.Column)
	for i := range current.Columns {
		currentCols[current.Columns[i].Name] = &current.Columns[i]
	}

	desiredCols := make(map[string]*database.Column)
	for i := range desired.Columns {
		desiredCols[desired.Columns[i].Name] = &desired.Columns[i]
	}

	// Find added and modified columns
	for name, desiredCol := range desiredCols {
		currentCol, exists := currentCols[name]
		if !exists {
			// Column added
			diff.AddedColumns = append(diff.AddedColumns, *desiredCol)
		} else {
			// Column exists, check for modifications
			colDiff := diffColumns(currentCol, desiredCol)
			if colDiff != nil {
				diff.ModifiedColumns = append(diff.ModifiedColumns, *colDiff)
			}
		}
	}

	// Find removed columns
	for name, currentCol := range currentCols {
		if _, exists := desiredCols[name]; !exists {
			diff.RemovedColumns = append(diff.RemovedColumns, *currentCol)
		}
	}

	// Build maps for indexes
	currentIdxs := make(map[string]*database.Index)
	for i := range current.Indexes {
		currentIdxs[current.Indexes[i].Name] = &current.Indexes[i]
	}

	desiredIdxs := make(map[string]*database.Index)
	for i := range desired.Indexes {
		desiredIdxs[desired.Indexes[i].Name] = &desired.Indexes[i]
	}

	// Find added indexes
	for name, desiredIdx := range desiredIdxs {
		if _, exists := currentIdxs[name]; !exists {
			diff.AddedIndexes = append(diff.AddedIndexes, *desiredIdx)
		}
	}

	// Find removed indexes
	for name, currentIdx := range currentIdxs {
		if _, exists := desiredIdxs[name]; !exists {
			diff.RemovedIndexes = append(diff.RemovedIndexes, *currentIdx)
		}
	}

	// Build maps for foreign keys
	currentFKs := make(map[string]*database.ForeignKey)
	for i := range current.ForeignKeys {
		currentFKs[current.ForeignKeys[i].Name] = &current.ForeignKeys[i]
	}

	desiredFKs := make(map[string]*database.ForeignKey)
	for i := range desired.ForeignKeys {
		desiredFKs[desired.ForeignKeys[i].Name] = &desired.ForeignKeys[i]
	}

	// Find added foreign keys
	for name, desiredFK := range desiredFKs {
		if _, exists := currentFKs[name]; !exists {
			diff.AddedForeignKeys = append(diff.AddedForeignKeys, *desiredFK)
		}
	}

	// Find removed foreign keys
	for name, currentFK := range currentFKs {
		if _, exists := desiredFKs[name]; !exists {
			diff.RemovedForeignKeys = append(diff.RemovedForeignKeys, *currentFK)
		}
	}

	// Check for RLS changes
	if current.RLSEnabled != desired.RLSEnabled {
		diff.RLSChanged = true
		diff.RLSEnabled = desired.RLSEnabled
	}

	return diff
}

// diffColumns compares two columns and returns their differences
func diffColumns(current, desired *database.Column) *ColumnDiff {
	var changes []string

	if current.LogicalType() != desired.LogicalType() {
		changes = append(changes, "type")
	}
	if current.Nullable != desired.Nullable {
		changes = append(changes, "nullable")
	}
	if !equalDefaults(current.Default, desired.Default) {
		changes = append(changes, "default")
	}
	if current.IsPrimaryKey != desired.IsPrimaryKey {
		changes = append(changes, "is_primary_key")
	}

	if len(changes) == 0 {
		return nil
	}

	return &ColumnDiff{
		ColumnName: current.Name,
		Old:        *current,
		New:        *desired,
		Changes:    changes,
	}
}

// equalDefaults compares two default values
func equalDefaults(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// IsEmpty returns true if there are no differences
func (d *TableDiff) IsEmpty() bool {
	return len(d.AddedColumns) == 0 &&
		len(d.RemovedColumns) == 0 &&
		len(d.ModifiedColumns) == 0 &&
		len(d.AddedIndexes) == 0 &&
		len(d.RemovedIndexes) == 0 &&
		len(d.AddedForeignKeys) == 0 &&
		len(d.RemovedForeignKeys) == 0
}

// IsEmpty returns true if there are no differences
func (d *SchemaDiff) IsEmpty() bool {
	return len(d.AddedTables) == 0 &&
		len(d.RemovedTables) == 0 &&
		len(d.ModifiedTables) == 0
}
