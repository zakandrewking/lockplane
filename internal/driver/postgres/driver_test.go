package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/lockplane/lockplane/internal/database"
)

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
	// TODO FILO right?
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
	tables, err := GetTables(ctx, db, "public")
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	// Should have at least our test table
	found := false
	for _, table := range tables {
		if table.Name == "test_introspect_tables" {
			found = true
			if table.Schema != "public" {
				t.Errorf("Expected schema public. Got: %v", table.Schema)
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected to find test_introspect_tables in results, got: %v", tables)
	}
}
