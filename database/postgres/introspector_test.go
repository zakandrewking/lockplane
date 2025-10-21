package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/lockplane/lockplane/database"
)

// getTestDB returns a test database connection or skips the test if unavailable
func getTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use environment variable or default
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping test: cannot open database: %v", err)
	}

	// Check if database is actually reachable
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("Skipping test: database not available: %v", err)
	}

	return db
}

func TestIntrospector_GetTables(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create a test table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_introspect_tables (
			id integer PRIMARY KEY
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS test_introspect_tables")

	// Get tables
	tables, err := introspector.GetTables(ctx, db)
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	// Should have at least our test table
	found := false
	for _, table := range tables {
		if table == "test_introspect_tables" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find test_introspect_tables in results, got: %v", tables)
	}
}

func TestIntrospector_GetColumns(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	introspector := NewIntrospector()

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
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS test_introspect_columns")

	// Get columns
	columns, err := introspector.GetColumns(ctx, db, "test_introspect_columns")
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
	}
	if !ageCol.Nullable {
		t.Error("Expected age to be nullable")
	}

	// Check created_at column
	createdCol := findColumn(columns, "created_at")
	if createdCol == nil {
		t.Fatal("Expected to find 'created_at' column")
	}
	if createdCol.Default == nil {
		t.Error("Expected created_at to have a default value")
	}
}

func TestIntrospector_GetIndexes(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create a test table with indexes
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_introspect_indexes (
			id integer PRIMARY KEY,
			email text,
			username text
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS test_introspect_indexes")

	// Create a unique index
	_, err = db.ExecContext(ctx, "CREATE UNIQUE INDEX test_idx_email ON test_introspect_indexes (email)")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Get indexes
	indexes, err := introspector.GetIndexes(ctx, db, "test_introspect_indexes")
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	// Should have at least the test index (and the primary key index)
	found := false
	for _, idx := range indexes {
		if idx.Name == "test_idx_email" {
			found = true
			if !idx.Unique {
				t.Error("Expected test_idx_email to be unique")
			}
		}
	}

	if !found {
		t.Error("Expected to find test_idx_email index")
	}
}

func TestIntrospector_GetForeignKeys(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create parent table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_fk_users (
			id integer PRIMARY KEY
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create parent table: %v", err)
	}
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS test_fk_posts, test_fk_users CASCADE")

	// Create child table with foreign key
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_fk_posts (
			id integer PRIMARY KEY,
			user_id integer,
			CONSTRAINT fk_test_user_id FOREIGN KEY (user_id)
				REFERENCES test_fk_users (id)
				ON DELETE CASCADE
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create child table: %v", err)
	}

	// Get foreign keys
	fks, err := introspector.GetForeignKeys(ctx, db, "test_fk_posts")
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	if len(fks) != 1 {
		t.Fatalf("Expected 1 foreign key, got %d", len(fks))
	}

	fk := fks[0]
	if fk.Name != "fk_test_user_id" {
		t.Errorf("Expected FK name 'fk_test_user_id', got '%s'", fk.Name)
	}

	if len(fk.Columns) != 1 || fk.Columns[0] != "user_id" {
		t.Errorf("Expected columns [user_id], got %v", fk.Columns)
	}

	if fk.ReferencedTable != "test_fk_users" {
		t.Errorf("Expected referenced table 'test_fk_users', got '%s'", fk.ReferencedTable)
	}

	if len(fk.ReferencedColumns) != 1 || fk.ReferencedColumns[0] != "id" {
		t.Errorf("Expected referenced columns [id], got %v", fk.ReferencedColumns)
	}

	if fk.OnDelete == nil || *fk.OnDelete != "CASCADE" {
		t.Errorf("Expected OnDelete 'CASCADE', got %v", fk.OnDelete)
	}
}

func TestIntrospector_IntrospectSchema(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create a test schema
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_introspect_schema (
			id integer PRIMARY KEY,
			name text NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS test_introspect_schema")

	// Introspect full schema
	schema, err := introspector.IntrospectSchema(ctx, db)
	if err != nil {
		t.Fatalf("IntrospectSchema failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}

	// Should have at least our test table
	found := false
	for _, table := range schema.Tables {
		if table.Name == "test_introspect_schema" {
			found = true
			if len(table.Columns) != 2 {
				t.Errorf("Expected 2 columns, got %d", len(table.Columns))
			}
		}
	}

	if !found {
		t.Error("Expected to find test_introspect_schema in schema")
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
