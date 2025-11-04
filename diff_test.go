package main

import (
	"testing"

	"github.com/lockplane/lockplane/database"
)

// loadSchemaFixture loads a schema from a JSON file
func loadSchemaFixture(t *testing.T, path string) *Schema {
	t.Helper()

	schema, err := LoadJSONSchema(path)
	if err != nil {
		t.Fatalf("Failed to load JSON schema from %s: %v", path, err)
	}

	return schema
}

func TestDiffSchemas_UsesLogicalTypes(t *testing.T) {
	before := &Schema{
		Tables: []Table{
			{
				Name: "todos",
				Columns: []Column{
					{
						Name:     "completed",
						Type:     "INTEGER",
						Nullable: false,
						TypeMetadata: &database.TypeMetadata{
							Logical: "integer",
							Raw:     "INTEGER",
							Dialect: database.DialectSQLite,
						},
					},
				},
			},
		},
	}

	after := &Schema{
		Tables: []Table{
			{
				Name: "todos",
				Columns: []Column{
					{
						Name:     "completed",
						Type:     "integer",
						Nullable: false,
						TypeMetadata: &database.TypeMetadata{
							Logical: "integer",
							Raw:     "pg_catalog.int4",
							Dialect: database.DialectPostgres,
						},
					},
				},
			},
		},
	}

	diff := DiffSchemas(before, after)
	if !diff.IsEmpty() {
		t.Fatalf("expected diff to be empty when logical types match, got %#v", diff)
	}
}

func TestDiffSchemas_AddTable(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	before := loadSchemaFixture(t, "testdata/diffs/add_table/before.json")
	after := loadSchemaFixture(t, "testdata/diffs/add_table/after.json")

	diff := DiffSchemas(before, after)

	if len(diff.AddedTables) != 1 {
		t.Errorf("Expected 1 added table, got %d", len(diff.AddedTables))
	}

	if len(diff.AddedTables) > 0 && diff.AddedTables[0].Name != "posts" {
		t.Errorf("Expected added table 'posts', got '%s'", diff.AddedTables[0].Name)
	}

	if len(diff.RemovedTables) != 0 {
		t.Errorf("Expected 0 removed tables, got %d", len(diff.RemovedTables))
	}

	if len(diff.ModifiedTables) != 0 {
		t.Errorf("Expected 0 modified tables, got %d", len(diff.ModifiedTables))
	}
}

func TestDiffSchemas_AddColumn(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	before := loadSchemaFixture(t, "testdata/diffs/add_column/before.json")
	after := loadSchemaFixture(t, "testdata/diffs/add_column/after.json")

	diff := DiffSchemas(before, after)

	if len(diff.ModifiedTables) != 1 {
		t.Fatalf("Expected 1 modified table, got %d", len(diff.ModifiedTables))
	}

	tableDiff := diff.ModifiedTables[0]
	if tableDiff.TableName != "users" {
		t.Errorf("Expected table 'users', got '%s'", tableDiff.TableName)
	}

	if len(tableDiff.AddedColumns) != 1 {
		t.Fatalf("Expected 1 added column, got %d", len(tableDiff.AddedColumns))
	}

	if tableDiff.AddedColumns[0].Name != "email" {
		t.Errorf("Expected column 'email', got '%s'", tableDiff.AddedColumns[0].Name)
	}
}

func TestDiffSchemas_RemoveColumn(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	before := loadSchemaFixture(t, "testdata/diffs/remove_column/before.json")
	after := loadSchemaFixture(t, "testdata/diffs/remove_column/after.json")

	diff := DiffSchemas(before, after)

	if len(diff.ModifiedTables) != 1 {
		t.Fatalf("Expected 1 modified table, got %d", len(diff.ModifiedTables))
	}

	tableDiff := diff.ModifiedTables[0]
	if len(tableDiff.RemovedColumns) != 1 {
		t.Fatalf("Expected 1 removed column, got %d", len(tableDiff.RemovedColumns))
	}

	if tableDiff.RemovedColumns[0].Name != "deprecated_field" {
		t.Errorf("Expected column 'deprecated_field', got '%s'", tableDiff.RemovedColumns[0].Name)
	}
}

func TestDiffSchemas_ModifyColumn(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	before := loadSchemaFixture(t, "testdata/diffs/modify_column/before.json")
	after := loadSchemaFixture(t, "testdata/diffs/modify_column/after.json")

	diff := DiffSchemas(before, after)

	if len(diff.ModifiedTables) != 1 {
		t.Fatalf("Expected 1 modified table, got %d", len(diff.ModifiedTables))
	}

	tableDiff := diff.ModifiedTables[0]
	if len(tableDiff.ModifiedColumns) != 1 {
		t.Fatalf("Expected 1 modified column, got %d", len(tableDiff.ModifiedColumns))
	}

	colDiff := tableDiff.ModifiedColumns[0]
	if colDiff.ColumnName != "email" {
		t.Errorf("Expected column 'email', got '%s'", colDiff.ColumnName)
	}

	if len(colDiff.Changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(colDiff.Changes))
	}

	if colDiff.Changes[0] != "nullable" {
		t.Errorf("Expected change 'nullable', got '%s'", colDiff.Changes[0])
	}

	if colDiff.Old.Nullable != true {
		t.Error("Expected old nullable to be true")
	}

	if colDiff.New.Nullable != false {
		t.Error("Expected new nullable to be false")
	}
}

func TestDiffSchemas_AddIndex(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	before := loadSchemaFixture(t, "testdata/diffs/add_index/before.json")
	after := loadSchemaFixture(t, "testdata/diffs/add_index/after.json")

	diff := DiffSchemas(before, after)

	if len(diff.ModifiedTables) != 1 {
		t.Fatalf("Expected 1 modified table, got %d", len(diff.ModifiedTables))
	}

	tableDiff := diff.ModifiedTables[0]
	if len(tableDiff.AddedIndexes) != 1 {
		t.Fatalf("Expected 1 added index, got %d", len(tableDiff.AddedIndexes))
	}

	if tableDiff.AddedIndexes[0].Name != "idx_users_email" {
		t.Errorf("Expected index 'idx_users_email', got '%s'", tableDiff.AddedIndexes[0].Name)
	}

	if !tableDiff.AddedIndexes[0].Unique {
		t.Error("Expected index to be unique")
	}
}

func TestDiffSchemas_NoChanges(t *testing.T) {
	t.Skip("JSON test fixtures not yet created")
	schema := loadSchemaFixture(t, "testdata/diffs/add_table/before.json")

	diff := DiffSchemas(schema, schema)

	if !diff.IsEmpty() {
		t.Error("Expected diff to be empty when comparing schema with itself")
	}
}

func TestSchemaDiff_IsEmpty(t *testing.T) {
	emptyDiff := &SchemaDiff{}
	if !emptyDiff.IsEmpty() {
		t.Error("Expected empty diff to report as empty")
	}

	nonEmptyDiff := &SchemaDiff{
		AddedTables: []Table{{Name: "test"}},
	}
	if nonEmptyDiff.IsEmpty() {
		t.Error("Expected non-empty diff to report as not empty")
	}
}

func TestTableDiff_IsEmpty(t *testing.T) {
	emptyDiff := &TableDiff{TableName: "test"}
	if !emptyDiff.IsEmpty() {
		t.Error("Expected empty table diff to report as empty")
	}

	nonEmptyDiff := &TableDiff{
		TableName:    "test",
		AddedColumns: []Column{{Name: "col"}},
	}
	if nonEmptyDiff.IsEmpty() {
		t.Error("Expected non-empty table diff to report as not empty")
	}
}
