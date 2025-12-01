package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/lockplane/lockplane/internal/database"
)

var defaultSchema = "public"

// getTestDB returns a test database connection or skips the test if unavailable
func getTestDb(t *testing.T) (*sql.DB, *Driver) {
	t.Helper()

	// see DEVELOPMENT.md
	dbUrl := os.Getenv("POSTGRES_URL")

	driver := NewDriver()
	db, err := driver.OpenConnection(database.ConnectionConfig{
		PostgresUrl: dbUrl,
	})
	if err != nil {
		t.Skipf("Skipping test: cannot open database: %v", err)
	}

	return db, driver
}

func TestDriver_Name(t *testing.T) {
	driver := NewDriver()

	if driver.Name() != "postgres" {
		t.Errorf("Expected name 'postgres', got '%s'", driver.Name())
	}
}

func TestDriver_GetTables(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Create a test table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE test_introspect_tables (
			id integer PRIMARY KEY
		)
	`)

	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE test_introspect_tables") }()

	// Get tables
	tables, err := GetTables(ctx, db, defaultSchema)
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	// Should have at least our test table
	found := false
	for _, table := range tables {
		if table.Name == "test_introspect_tables" {
			found = true
			if table.Schema != defaultSchema {
				t.Errorf("Expected schema %v. Got: %v", defaultSchema, table.Schema)
			}
			if len(table.Columns) != 1 {
				t.Errorf("Expected 1 column. Found: %v", len(table.Columns))
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected to find test_introspect_tables in results, got: %v", tables)
	}
}

// Helper function to find a column by name
func findColumn(columns []database.Column, name string) *database.Column {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}

func TestIntrospector_GetColumns(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	// TODO test a bunch of different types, defaults, etc (SERIAL) see
	// lockplane-vibe/devdocs/unsupported-features.md

	// Create a test table with various column types
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_introspect_columns (
			id integer PRIMARY KEY,
			name text NOT NULL,
			age integer,
			created_at timestamp DEFAULT now()
		)
	`)

	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS test_introspect_columns") }()

	// Get columns
	columns, err := GetColumns(ctx, db, defaultSchema, "test_introspect_columns")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 4 {
		t.Errorf("Expected 4 columns, got %d", len(columns))
	}

	// Check id column
	idCol := findColumn(columns, "id")
	if idCol == nil {
		t.Fatal("Expected to find 'id' column")
		return
	}
	if idCol.Type != "integer" {
		t.Errorf("Expected id type 'integer', got '%s'", idCol.Type)
	}
	if idCol.Nullable {
		t.Error("Expected id to be NOT NULL")
	}
	if !idCol.IsPrimaryKey {
		t.Error("Expected id to be primary key")
	}

	// Check name column
	nameCol := findColumn(columns, "name")
	if nameCol == nil {
		t.Fatal("Expected to find 'name' column")
		return
	}
	if nameCol.Type != "text" {
		t.Errorf("Expected name type 'text', got '%s'", nameCol.Type)
	}
	if nameCol.Nullable {
		t.Error("Expected name to be NOT NULL")
	}

	// Check age column
	ageCol := findColumn(columns, "age")
	if ageCol == nil {
		t.Fatal("Expected to find 'age' column")
		return
	}
	if !ageCol.Nullable {
		t.Error("Expected age to be nullable")
	}

	// Check created_at column
	createdCol := findColumn(columns, "created_at")
	if createdCol == nil {
		t.Fatal("Expected to find 'created_at' column")
		return
	}
	if createdCol.Default == nil {
		t.Error("Expected created_at to have a default value")
	}
}
