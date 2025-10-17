package main

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

func TestIntrospectSchema(t *testing.T) {
	// Connect to test database
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Database not available (this is okay in CI): %v", err)
	}

	// Clean up any existing test tables
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS test_table CASCADE")

	// Create a test table with various column types
	_, err = db.ExecContext(ctx, `
		CREATE TABLE test_table (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE,
			age INTEGER,
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS test_table CASCADE")
	}()

	// Create an additional index
	_, err = db.ExecContext(ctx, `
		CREATE INDEX idx_test_name ON test_table(name)
	`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Run introspection
	schema, err := introspectSchema(ctx, db)
	if err != nil {
		t.Fatalf("Failed to introspect schema: %v", err)
	}

	// Verify results
	if schema == nil {
		t.Fatal("Expected schema to be non-nil")
	}

	// Find test_table
	var testTable *Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == "test_table" {
			testTable = &schema.Tables[i]
			break
		}
	}

	if testTable == nil {
		t.Fatal("Expected to find test_table in schema")
	}

	// Verify table name
	if testTable.Name != "test_table" {
		t.Errorf("Expected table name 'test_table', got '%s'", testTable.Name)
	}

	// Verify columns
	expectedColumns := map[string]struct {
		Type         string
		Nullable     bool
		IsPrimaryKey bool
	}{
		"id":         {"integer", false, true},
		"name":       {"text", false, false},
		"email":      {"text", true, false},
		"age":        {"integer", true, false},
		"active":     {"boolean", true, false},
		"created_at": {"timestamp without time zone", true, false},
	}

	if len(testTable.Columns) != len(expectedColumns) {
		t.Errorf("Expected %d columns, got %d", len(expectedColumns), len(testTable.Columns))
	}

	for _, col := range testTable.Columns {
		expected, ok := expectedColumns[col.Name]
		if !ok {
			t.Errorf("Unexpected column: %s", col.Name)
			continue
		}

		if col.Type != expected.Type {
			t.Errorf("Column %s: expected type '%s', got '%s'", col.Name, expected.Type, col.Type)
		}

		if col.Nullable != expected.Nullable {
			t.Errorf("Column %s: expected nullable=%v, got %v", col.Name, expected.Nullable, col.Nullable)
		}

		if col.IsPrimaryKey != expected.IsPrimaryKey {
			t.Errorf("Column %s: expected is_primary_key=%v, got %v", col.Name, expected.IsPrimaryKey, col.IsPrimaryKey)
		}

		// Verify defaults exist for certain columns
		if col.Name == "id" && col.Default == nil {
			t.Errorf("Column %s: expected default to be set", col.Name)
		}
		if col.Name == "active" && col.Default == nil {
			t.Errorf("Column %s: expected default to be set", col.Name)
		}
		if col.Name == "created_at" && col.Default == nil {
			t.Errorf("Column %s: expected default to be set", col.Name)
		}
	}

	// Verify indexes (at least primary key and unique constraint should exist)
	if len(testTable.Indexes) < 2 {
		t.Errorf("Expected at least 2 indexes (primary key + unique/index), got %d", len(testTable.Indexes))
	}

	// Check for specific indexes
	foundPrimaryKey := false
	foundUniqueEmail := false
	foundNameIndex := false

	for _, idx := range testTable.Indexes {
		if idx.Name == "test_table_pkey" {
			foundPrimaryKey = true
			if !idx.Unique {
				t.Error("Primary key index should be marked as unique")
			}
		}
		if idx.Name == "test_table_email_key" {
			foundUniqueEmail = true
			if !idx.Unique {
				t.Error("Unique constraint index should be marked as unique")
			}
		}
		if idx.Name == "idx_test_name" {
			foundNameIndex = true
			if idx.Unique {
				t.Error("Regular index should not be marked as unique")
			}
		}
	}

	if !foundPrimaryKey {
		t.Error("Expected to find primary key index")
	}
	if !foundUniqueEmail {
		t.Error("Expected to find unique constraint index for email")
	}
	if !foundNameIndex {
		t.Error("Expected to find regular index on name")
	}
}

func TestGetColumns(t *testing.T) {
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Database not available (this is okay in CI): %v", err)
	}

	// Clean up and create test table
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS col_test CASCADE")
	_, err = db.ExecContext(ctx, `
		CREATE TABLE col_test (
			id SERIAL PRIMARY KEY,
			required_field TEXT NOT NULL,
			optional_field TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS col_test CASCADE")
	}()

	// Get columns
	columns, err := getColumns(ctx, db, "col_test")
	if err != nil {
		t.Fatalf("Failed to get columns: %v", err)
	}

	if len(columns) != 3 {
		t.Fatalf("Expected 3 columns, got %d", len(columns))
	}

	// Verify column order (should match ordinal_position)
	if columns[0].Name != "id" {
		t.Errorf("Expected first column to be 'id', got '%s'", columns[0].Name)
	}
	if columns[1].Name != "required_field" {
		t.Errorf("Expected second column to be 'required_field', got '%s'", columns[1].Name)
	}
	if columns[2].Name != "optional_field" {
		t.Errorf("Expected third column to be 'optional_field', got '%s'", columns[2].Name)
	}
}

func TestGetIndexes(t *testing.T) {
	connStr := getEnv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Database not available (this is okay in CI): %v", err)
	}

	// Clean up and create test table
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS idx_test CASCADE")
	_, err = db.ExecContext(ctx, `
		CREATE TABLE idx_test (
			id SERIAL PRIMARY KEY,
			email TEXT UNIQUE,
			name TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	_, err = db.ExecContext(ctx, `CREATE INDEX idx_name ON idx_test(name)`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	defer func() {
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS idx_test CASCADE")
	}()

	// Get indexes
	indexes, err := getIndexes(ctx, db, "idx_test")
	if err != nil {
		t.Fatalf("Failed to get indexes: %v", err)
	}

	if len(indexes) < 2 {
		t.Fatalf("Expected at least 2 indexes, got %d", len(indexes))
	}

	// Verify we can distinguish unique from non-unique indexes
	foundUnique := false
	foundNonUnique := false

	for _, idx := range indexes {
		if idx.Unique {
			foundUnique = true
		} else {
			foundNonUnique = true
		}
	}

	if !foundUnique {
		t.Error("Expected to find at least one unique index")
	}
	if !foundNonUnique {
		t.Error("Expected to find at least one non-unique index")
	}
}
