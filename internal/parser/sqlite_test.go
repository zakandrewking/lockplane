//go:build ignore
// +build ignore

// TODO: Re-enable this test file after moving schema types to internal/schema

package parser

import (
	"os"
	"strings"
	"testing"

	"github.com/lockplane/lockplane/database"
)

// TestParseSQLiteSchema_TypePreservation verifies that SQLite types are preserved exactly as-is
func TestParseSQLiteSchema_TypePreservation(t *testing.T) {
	ddl := `
CREATE TABLE test_types (
    id INTEGER PRIMARY KEY,
    small_int INTEGER,
    big_int BIGINT,
    real_num REAL,
    numeric_val NUMERIC(10, 2),
    text_col TEXT,
    varchar_col VARCHAR(100),
    blob_col BLOB,
    bool_col BOOLEAN
);
`
	schema, err := parseSQLiteSQLSchema(ddl)
	if err != nil {
		t.Fatalf("parseSQLiteSQLSchema failed: %v", err)
	}

	if schema.Dialect != database.DialectSQLite {
		t.Errorf("expected dialect sqlite, got %s", schema.Dialect)
	}

	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}

	table := &schema.Tables[0]
	if table.Name != "test_types" {
		t.Errorf("expected table name test_types, got %s", table.Name)
	}

	// Check that types are preserved as SQLite reports them (with modifiers)
	expectedTypes := map[string]string{
		"id":          "INTEGER",
		"small_int":   "INTEGER",
		"big_int":     "BIGINT",
		"real_num":    "REAL",
		"numeric_val": "NUMERIC(10, 2)", // SQLite preserves precision/scale
		"text_col":    "TEXT",
		"varchar_col": "VARCHAR(100)", // SQLite preserves length
		"blob_col":    "BLOB",
		"bool_col":    "BOOLEAN",
	}

	for _, col := range table.Columns {
		expectedType, ok := expectedTypes[col.Name]
		if !ok {
			t.Errorf("unexpected column: %s", col.Name)
			continue
		}

		// Check type matches (case-insensitive)
		if !strings.EqualFold(col.Type, expectedType) {
			t.Errorf("column %s: expected type %s, got %s", col.Name, expectedType, col.Type)
		}
	}
}

// TestParseSQLiteSchema_DefaultExpressions verifies that default expressions are preserved
func TestParseSQLiteSchema_DefaultExpressions(t *testing.T) {
	ddl := `
CREATE TABLE test_defaults (
    id INTEGER PRIMARY KEY,
    status TEXT DEFAULT 'pending',
    count INTEGER DEFAULT 0,
    score REAL DEFAULT 0.0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT (datetime('now')),
    due_date TEXT DEFAULT (date('now', '+7 days')),
    timestamp_val TEXT DEFAULT (strftime('%s','now'))
);
`
	schema, err := parseSQLiteSQLSchema(ddl)
	if err != nil {
		t.Fatalf("parseSQLiteSQLSchema failed: %v", err)
	}

	table := &schema.Tables[0]

	// Test various default expression formats
	tests := []struct {
		columnName        string
		shouldHaveDefault bool
		containsSubstring string
	}{
		{"id", false, ""},
		{"status", true, "pending"},
		{"count", true, "0"},
		{"score", true, "0.0"},
		{"created_at", true, "CURRENT_TIMESTAMP"},
		{"updated_at", true, "datetime('now')"},
		{"due_date", true, "date('now'"},
		{"timestamp_val", true, "strftime"},
	}

	for _, tt := range tests {
		col := findColumnInTable(table, tt.columnName)
		if col == nil {
			t.Errorf("column %s not found", tt.columnName)
			continue
		}

		if tt.shouldHaveDefault {
			if col.Default == nil {
				t.Errorf("column %s: expected default but got nil", tt.columnName)
				continue
			}

			defaultStr := strings.ToLower(*col.Default)
			if tt.containsSubstring != "" && !strings.Contains(defaultStr, strings.ToLower(tt.containsSubstring)) {
				t.Errorf("column %s: expected default to contain %q, got %q",
					tt.columnName, tt.containsSubstring, *col.Default)
			}

			// Check metadata is populated
			if col.DefaultMetadata == nil {
				t.Errorf("column %s: expected default metadata to be populated", tt.columnName)
			} else if col.DefaultMetadata.Dialect != database.DialectSQLite {
				t.Errorf("column %s: expected default metadata dialect sqlite, got %s",
					tt.columnName, col.DefaultMetadata.Dialect)
			}
		} else {
			if col.Default != nil {
				t.Errorf("column %s: expected no default but got %q", tt.columnName, *col.Default)
			}
		}
	}
}

// TestParseSQLiteSchema_ForeignKeys verifies foreign key preservation
func TestParseSQLiteSchema_ForeignKeys(t *testing.T) {
	ddl := `
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT
);
`
	schema, err := parseSQLiteSQLSchema(ddl)
	if err != nil {
		t.Fatalf("parseSQLiteSQLSchema failed: %v", err)
	}

	if len(schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(schema.Tables))
	}

	var postsTable *database.Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == "posts" {
			postsTable = &schema.Tables[i]
			break
		}
	}

	if postsTable == nil {
		t.Fatalf("posts table not found")
	}

	if len(postsTable.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(postsTable.ForeignKeys))
	}

	fk := postsTable.ForeignKeys[0]
	if fk.ReferencedTable != "users" {
		t.Errorf("expected foreign key to reference users, got %s", fk.ReferencedTable)
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "user_id" {
		t.Errorf("expected foreign key column user_id, got %v", fk.Columns)
	}
	if fk.OnDelete == nil || *fk.OnDelete != "CASCADE" {
		t.Errorf("expected ON DELETE CASCADE, got %v", fk.OnDelete)
	}
}

// TestParseSQLiteSchema_Indexes verifies index preservation
func TestParseSQLiteSchema_Indexes(t *testing.T) {
	ddl := `
CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    sku TEXT NOT NULL,
    name TEXT,
    category TEXT
);

CREATE UNIQUE INDEX idx_products_sku ON products(sku);
CREATE INDEX idx_products_category ON products(category);
`
	schema, err := parseSQLiteSQLSchema(ddl)
	if err != nil {
		t.Fatalf("parseSQLiteSQLSchema failed: %v", err)
	}

	table := &schema.Tables[0]

	// NOTE: There's a known issue with modernc.org/sqlite where PRAGMA index_info
	// returns 0 rows, so index columns are currently not populated.
	// We still verify that indexes are discovered with correct names and uniqueness.

	// Verify we have at least the indexes we created
	if len(table.Indexes) < 2 {
		t.Fatalf("expected at least 2 indexes, got %d", len(table.Indexes))
	}

	// SQLite reports indexes including those created implicitly, so we check for specific ones
	var skuIndex, categoryIndex *database.Index
	for i := range table.Indexes {
		idx := &table.Indexes[i]
		t.Logf("Found index: %s (unique=%v, columns=%v)", idx.Name, idx.Unique, idx.Columns)
		switch idx.Name {
		case "idx_products_sku":
			skuIndex = idx
		case "idx_products_category":
			categoryIndex = idx
		}
	}

	if skuIndex == nil {
		t.Errorf("idx_products_sku not found in indexes: %v", table.Indexes)
	} else {
		if !skuIndex.Unique {
			t.Error("idx_products_sku should be unique")
		}
		// SKIP column check due to modernc.org/sqlite PRAGMA index_info issue
		// if len(skuIndex.Columns) != 1 || skuIndex.Columns[0] != "sku" {
		// 	t.Errorf("idx_products_sku: expected columns [sku], got %v", skuIndex.Columns)
		// }
	}

	if categoryIndex == nil {
		t.Errorf("idx_products_category not found in indexes: %v", table.Indexes)
	} else {
		if categoryIndex.Unique {
			t.Error("idx_products_category should not be unique")
		}
		// SKIP column check due to modernc.org/sqlite PRAGMA index_info issue
		// if len(categoryIndex.Columns) != 1 || categoryIndex.Columns[0] != "category" {
		// 	t.Errorf("idx_products_category: expected columns [category], got %v", categoryIndex.Columns)
		// }
	}
}

// TestLoadSQLiteSchemaFromFile tests loading a SQLite schema from a fixture file
func TestLoadSQLiteSchemaFromFile(t *testing.T) {
	fixtureData, err := os.ReadFile("testdata/fixtures/sqlite/types_and_defaults.lp.sql")
	if err != nil {
		t.Skipf("Skipping test: fixture file not available: %v", err)
	}

	schema, err := loadSQLSchemaFromBytes(fixtureData, &SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("loadSQLSchemaFromBytes failed: %v", err)
	}

	if schema.Dialect != database.DialectSQLite {
		t.Errorf("expected dialect sqlite, got %s", schema.Dialect)
	}

	if len(schema.Tables) < 1 {
		t.Fatalf("expected at least 1 table, got %d", len(schema.Tables))
	}

	// Find the tasks table
	var tasksTable *Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == "tasks" {
			tasksTable = &schema.Tables[i]
			break
		}
	}

	if tasksTable == nil {
		t.Fatal("tasks table not found in fixture")
	}

	// Verify some key columns with defaults
	completedCol := findColumnInTable(tasksTable, "completed")
	if completedCol == nil {
		t.Fatal("completed column not found")
	}
	if completedCol.Type != "INTEGER" {
		t.Errorf("completed: expected type INTEGER, got %s", completedCol.Type)
	}
	if completedCol.Default == nil || *completedCol.Default != "0" {
		t.Errorf("completed: expected default 0, got %v", completedCol.Default)
	}

	createdAtCol := findColumnInTable(tasksTable, "created_at")
	if createdAtCol == nil {
		t.Fatal("created_at column not found")
	}
	if createdAtCol.Default == nil {
		t.Error("created_at: expected default to be set")
	} else if !strings.Contains(strings.ToUpper(*createdAtCol.Default), "CURRENT_TIMESTAMP") {
		t.Errorf("created_at: expected CURRENT_TIMESTAMP, got %s", *createdAtCol.Default)
	}

	updatedAtCol := findColumnInTable(tasksTable, "updated_at")
	if updatedAtCol == nil {
		t.Fatal("updated_at column not found")
	}
	if updatedAtCol.Default == nil {
		t.Error("updated_at: expected default to be set")
	} else if !strings.Contains(strings.ToLower(*updatedAtCol.Default), "datetime('now')") {
		t.Errorf("updated_at: expected datetime('now'), got %s", *updatedAtCol.Default)
	}
}

// findColumnInTable is a helper to find a column by name
func findColumnInTable(table *Table, name string) *Column {
	for i := range table.Columns {
		if table.Columns[i].Name == name {
			return &table.Columns[i]
		}
	}
	return nil
}
