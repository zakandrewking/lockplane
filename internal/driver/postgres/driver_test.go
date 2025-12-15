package postgres

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/lockplane/lockplane/internal/database"
)

var defaultSchema = "public"

// getTestDB returns a test database connection or skips the test if unavailable
func getTestDb(t *testing.T) (*sql.DB, *Driver) {
	t.Helper()

	// Skip integration tests when running with -short flag
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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

func TestIntrospector_IntegerTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"SMALLINT", "SMALLINT", "smallint"},
		{"INTEGER", "INTEGER", "integer"},
		{"BIGINT", "BIGINT", "bigint"},
		{"INT", "INT", "integer"},    // INT is alias for INTEGER
		{"INT2", "INT2", "smallint"}, // INT2 is alias for SMALLINT
		{"INT4", "INT4", "integer"},  // INT4 is alias for INTEGER
		{"INT8", "INT8", "bigint"},   // INT8 is alias for BIGINT
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_int_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
			if !col.Nullable {
				t.Error("Expected column to be nullable by default")
			}
		})
	}
}

func TestIntrospector_SerialTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"SMALLSERIAL", "SMALLSERIAL", "smallserial"},
		{"SERIAL", "SERIAL", "serial"},
		{"BIGSERIAL", "BIGSERIAL", "bigserial"},
		{"SERIAL2", "SERIAL2", "smallserial"},
		{"SERIAL4", "SERIAL4", "serial"},
		{"SERIAL8", "SERIAL8", "bigserial"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_serial_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
			if col.Nullable {
				t.Error("Expected SERIAL column to be NOT NULL")
			}
			// SERIAL type implies nextval(), so default should be nil
			if col.Default != nil {
				t.Errorf("Expected SERIAL column to have no explicit default (type implies sequence), got %v", col.Default)
			}
		})
	}
}

// TestIntrospector_ManualSequence tests that columns with manual sequences (not owned by the column)
// are NOT detected as SERIAL types. This validates the stricter SERIAL detection.
func TestIntrospector_ManualSequence(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tableName := "test_manual_seq"

	// Create a manual sequence (not owned by any column)
	_, err := db.ExecContext(ctx, "CREATE SEQUENCE manual_seq")
	if err != nil {
		t.Fatalf("Failed to create sequence: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP SEQUENCE IF EXISTS manual_seq") }()

	// Create table with a column using the manual sequence
	// This should NOT be detected as SERIAL because the sequence is not owned by the column
	_, err = db.ExecContext(ctx, "CREATE TABLE "+tableName+" (id INTEGER NOT NULL DEFAULT nextval('manual_seq'::regclass))")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName) }()

	// Introspect
	columns, err := GetColumns(ctx, db, defaultSchema, tableName)
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 1 {
		t.Fatalf("Expected 1 column, got %d", len(columns))
	}

	col := columns[0]

	// Should remain as 'integer', NOT 'serial'
	if col.Type != "integer" {
		t.Errorf("Expected type 'integer' for manual sequence, got %q", col.Type)
	}

	// Should have the default value preserved (not nil)
	if col.Default == nil {
		t.Error("Expected default value to be preserved for manual sequence")
	} else if !strings.Contains(*col.Default, "nextval") {
		t.Errorf("Expected default to contain 'nextval', got %q", *col.Default)
	}
}

func TestIntrospector_FloatingPointTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"REAL", "REAL", "real"},
		{"FLOAT4", "FLOAT4", "real"},
		{"DOUBLE_PRECISION", "DOUBLE PRECISION", "double precision"},
		{"FLOAT8", "FLOAT8", "double precision"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_float_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
		})
	}
}

func TestIntrospector_CharacterTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"TEXT", "TEXT", "text"},
		{"VARCHAR", "VARCHAR", "character varying"},
		{"VARCHAR_50", "VARCHAR(50)", "character varying"},
		{"CHAR", "CHAR", "character"},
		{"CHAR_10", "CHAR(10)", "character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_char_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
		})
	}
}

func TestIntrospector_TimestampTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"TIMESTAMP", "TIMESTAMP", "timestamp without time zone"},
		{"TIMESTAMPTZ", "TIMESTAMPTZ", "timestamp with time zone"},
		{"TIMESTAMP_WITH_TIME_ZONE", "TIMESTAMP WITH TIME ZONE", "timestamp with time zone"},
		{"TIME", "TIME", "time without time zone"},
		{"TIMETZ", "TIMETZ", "time with time zone"},
		{"TIME_WITH_TIME_ZONE", "TIME WITH TIME ZONE", "time with time zone"},
		{"DATE", "DATE", "date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_time_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
		})
	}
}

func TestIntrospector_NumericTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"NUMERIC", "NUMERIC", "numeric"},
		{"NUMERIC_10_2", "NUMERIC(10,2)", "numeric"},
		{"DECIMAL", "DECIMAL", "numeric"},
		{"DECIMAL_8_4", "DECIMAL(8,4)", "numeric"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_numeric_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
		})
	}
}

func TestIntrospector_BooleanType(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"BOOLEAN", "BOOLEAN", "boolean"},
		{"BOOL", "BOOL", "boolean"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_bool_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
		})
	}
}

func TestIntrospector_SpecialTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"UUID", "UUID", "uuid"},
		{"JSON", "JSON", "json"},
		{"JSONB", "JSONB", "jsonb"},
		{"BYTEA", "BYTEA", "bytea"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_special_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
		})
	}
}

func TestIntrospector_ArrayTypes(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tests := []struct {
		name         string
		sqlType      string
		expectedType string
	}{
		{"INTEGER_ARRAY", "INTEGER[]", "ARRAY"},
		{"TEXT_ARRAY", "TEXT[]", "ARRAY"},
		{"VARCHAR_ARRAY", "VARCHAR(50)[]", "ARRAY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "test_array_" + strings.ToLower(tt.name)

			// Create table
			_, err := db.ExecContext(ctx, "CREATE TABLE "+tableName+" (col "+tt.sqlType+")")
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

			// Introspect
			columns, err := GetColumns(ctx, db, defaultSchema, tableName)
			if err != nil {
				t.Fatalf("GetColumns failed: %v", err)
			}

			if len(columns) != 1 {
				t.Fatalf("Expected 1 column, got %d", len(columns))
			}

			col := columns[0]
			if col.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, col.Type)
			}
		})
	}
}

func TestIntrospector_ConstraintsAndDefaults(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tableName := "test_constraints"

	// Create table with various constraints
	_, err := db.ExecContext(ctx, `
		CREATE TABLE `+tableName+` (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			age INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			is_active BOOLEAN DEFAULT true
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

	// Introspect
	columns, err := GetColumns(ctx, db, defaultSchema, tableName)
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 6 {
		t.Fatalf("Expected 6 columns, got %d", len(columns))
	}

	// Check id column
	id := findColumn(columns, "id")
	if id == nil {
		t.Fatal("Expected to find 'id' column")
	}
	if !id.IsPrimaryKey {
		t.Error("Expected id to be PRIMARY KEY")
	}
	if id.Nullable {
		t.Error("Expected id to be NOT NULL (implied by PRIMARY KEY)")
	}

	// Check email column
	email := findColumn(columns, "email")
	if email == nil {
		t.Fatal("Expected to find 'email' column")
	}
	if email.Nullable {
		t.Error("Expected email to be NOT NULL")
	}

	// Check age column with integer default
	age := findColumn(columns, "age")
	if age == nil {
		t.Fatal("Expected to find 'age' column")
	}
	if age.Default == nil {
		t.Error("Expected age to have default value")
	}

	// Check status column with string default
	status := findColumn(columns, "status")
	if status == nil {
		t.Fatal("Expected to find 'status' column")
	}
	if status.Default == nil {
		t.Error("Expected status to have default value")
	}

	// Check created_at column with function default
	createdAt := findColumn(columns, "created_at")
	if createdAt == nil {
		t.Fatal("Expected to find 'created_at' column")
	}
	if createdAt.Default == nil {
		t.Error("Expected created_at to have default value")
	}

	// Check is_active column with boolean default
	isActive := findColumn(columns, "is_active")
	if isActive == nil {
		t.Fatal("Expected to find 'is_active' column")
	}
	if isActive.Default == nil {
		t.Error("Expected is_active to have default value")
	}
}

func TestIntrospector_ComplexRealWorldTable(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tableName := "test_complex_users"

	// Create a complex real-world table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE `+tableName+` (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) NOT NULL,
			email VARCHAR(255) NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT,
			age INTEGER DEFAULT 0,
			balance NUMERIC(10,2) DEFAULT 0.00,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			tags TEXT[],
			metadata JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

	// Introspect
	columns, err := GetColumns(ctx, db, defaultSchema, tableName)
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 12 {
		t.Fatalf("Expected 12 columns, got %d", len(columns))
	}

	// Verify id column (SERIAL PRIMARY KEY)
	id := findColumn(columns, "id")
	if id == nil {
		t.Fatal("Expected to find 'id' column")
	}
	if id.Type != "serial" {
		t.Errorf("Expected id type 'serial', got %q", id.Type)
	}
	if !id.IsPrimaryKey {
		t.Error("Expected id to be PRIMARY KEY")
	}
	if id.Nullable {
		t.Error("Expected id to be NOT NULL")
	}

	// Verify username column (VARCHAR(50) NOT NULL)
	username := findColumn(columns, "username")
	if username == nil {
		t.Fatal("Expected to find 'username' column")
	}
	if username.Type != "character varying" {
		t.Errorf("Expected username type 'character varying', got %q", username.Type)
	}
	if username.Nullable {
		t.Error("Expected username to be NOT NULL")
	}

	// Verify tags column (TEXT[])
	tags := findColumn(columns, "tags")
	if tags == nil {
		t.Fatal("Expected to find 'tags' column")
	}
	if tags.Type != "ARRAY" {
		t.Errorf("Expected tags type 'ARRAY', got %q", tags.Type)
	}

	// Verify metadata column (JSONB)
	metadata := findColumn(columns, "metadata")
	if metadata == nil {
		t.Fatal("Expected to find 'metadata' column")
	}
	if metadata.Type != "jsonb" {
		t.Errorf("Expected metadata type 'jsonb', got %q", metadata.Type)
	}

	// Verify created_at column (TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)
	createdAt := findColumn(columns, "created_at")
	if createdAt == nil {
		t.Fatal("Expected to find 'created_at' column")
	}
	if createdAt.Nullable {
		t.Error("Expected created_at to be NOT NULL")
	}
	if createdAt.Default == nil {
		t.Error("Expected created_at to have default value")
	}
}

func TestGetRLSEnabled(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	// Create a test table without RLS
	_, err := db.ExecContext(ctx, `
		CREATE TABLE test_rls_disabled (
			id integer PRIMARY KEY
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE test_rls_disabled") }()

	// Test that RLS is disabled by default
	rlsEnabled, err := GetRLSEnabled(ctx, db, defaultSchema, "test_rls_disabled")
	if err != nil {
		t.Fatalf("GetRLSEnabled failed: %v", err)
	}
	if rlsEnabled {
		t.Error("Expected RLS to be disabled by default, but it was enabled")
	}

	// Enable RLS on the table
	_, err = db.ExecContext(ctx, "ALTER TABLE test_rls_disabled ENABLE ROW LEVEL SECURITY")
	if err != nil {
		t.Fatalf("Failed to enable RLS: %v", err)
	}

	// Test that RLS is now enabled
	rlsEnabled, err = GetRLSEnabled(ctx, db, defaultSchema, "test_rls_disabled")
	if err != nil {
		t.Fatalf("GetRLSEnabled failed: %v", err)
	}
	if !rlsEnabled {
		t.Error("Expected RLS to be enabled, but it was disabled")
	}

	// Disable RLS on the table
	_, err = db.ExecContext(ctx, "ALTER TABLE test_rls_disabled DISABLE ROW LEVEL SECURITY")
	if err != nil {
		t.Fatalf("Failed to disable RLS: %v", err)
	}

	// Test that RLS is now disabled again
	rlsEnabled, err = GetRLSEnabled(ctx, db, defaultSchema, "test_rls_disabled")
	if err != nil {
		t.Fatalf("GetRLSEnabled failed: %v", err)
	}
	if rlsEnabled {
		t.Error("Expected RLS to be disabled, but it was enabled")
	}
}

func TestGetRLSEnabledIntegrationWithGetTables(t *testing.T) {
	db, _ := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	// Create two test tables
	_, err := db.ExecContext(ctx, `
		CREATE TABLE test_rls_table1 (id integer PRIMARY KEY);
		CREATE TABLE test_rls_table2 (id integer PRIMARY KEY);
	`)
	if err != nil {
		t.Fatalf("Failed to create test tables: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DROP TABLE test_rls_table1")
		_, _ = db.ExecContext(ctx, "DROP TABLE test_rls_table2")
	}()

	// Enable RLS on table1 only
	_, err = db.ExecContext(ctx, "ALTER TABLE test_rls_table1 ENABLE ROW LEVEL SECURITY")
	if err != nil {
		t.Fatalf("Failed to enable RLS: %v", err)
	}

	// Get tables and verify RLS status
	tables, err := GetTables(ctx, db, defaultSchema)
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	// Find our test tables
	var table1, table2 *database.Table
	for i := range tables {
		switch tables[i].Name {
		case "test_rls_table1":
			table1 = &tables[i]
		case "test_rls_table2":
			table2 = &tables[i]
		}
	}

	if table1 == nil {
		t.Fatal("Expected to find test_rls_table1 in results")
	}
	if table2 == nil {
		t.Fatal("Expected to find test_rls_table2 in results")
	}

	// Verify RLS status
	if !table1.RLSEnabled {
		t.Error("Expected test_rls_table1 to have RLS enabled")
	}
	if table2.RLSEnabled {
		t.Error("Expected test_rls_table2 to have RLS disabled")
	}
}

func TestApplyMigration_Success(t *testing.T) {
	db, driver := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tableName := "test_apply_migration_success"

	// Ensure table doesn't exist
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName)

	// Apply migration to create table
	migration := `CREATE TABLE ` + tableName + ` (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		age INTEGER DEFAULT 0
	)`

	err := driver.ApplyMigration(ctx, db, migration)
	if err != nil {
		t.Fatalf("ApplyMigration failed: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

	// Verify table was created
	var exists bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = $1
			AND table_name = $2
		)
	`, defaultSchema, tableName).Scan(&exists)

	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}

	if !exists {
		t.Error("Expected table to exist after migration")
	}

	// Verify table structure
	columns, err := GetColumns(ctx, db, defaultSchema, tableName)
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(columns))
	}
}

func TestApplyMigration_Rollback(t *testing.T) {
	db, driver := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tableName := "test_apply_migration_rollback"

	// Ensure table doesn't exist
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName)

	// Apply migration with intentional error (invalid SQL)
	migration := `CREATE TABLE ` + tableName + ` (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL
	);
	INSERT INTO nonexistent_table VALUES (1);`

	err := driver.ApplyMigration(ctx, db, migration)
	if err == nil {
		t.Fatal("Expected ApplyMigration to fail with invalid SQL")
	}

	if !strings.Contains(err.Error(), "failed to execute migration") {
		t.Errorf("Expected error message to contain 'failed to execute migration', got: %v", err)
	}

	// Verify table was NOT created (transaction rolled back)
	var exists bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = $1
			AND table_name = $2
		)
	`, defaultSchema, tableName).Scan(&exists)

	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}

	if exists {
		t.Error("Expected table to NOT exist after failed migration (should have rolled back)")
		// Clean up
		_, _ = db.ExecContext(ctx, "DROP TABLE "+tableName)
	}
}

func TestApplyMigration_MultipleStatements(t *testing.T) {
	db, driver := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	table1 := "test_apply_migration_multi1"
	table2 := "test_apply_migration_multi2"

	// Ensure tables don't exist
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table1)
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table2)

	// Apply migration with multiple CREATE TABLE statements
	migration := `
		CREATE TABLE ` + table1 + ` (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);

		CREATE TABLE ` + table2 + ` (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL
		);
	`

	err := driver.ApplyMigration(ctx, db, migration)
	if err != nil {
		t.Fatalf("ApplyMigration failed: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "DROP TABLE "+table1)
		_, _ = db.ExecContext(ctx, "DROP TABLE "+table2)
	}()

	// Verify both tables were created
	for _, tableName := range []string{table1, table2} {
		var exists bool
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = $1
				AND table_name = $2
			)
		`, defaultSchema, tableName).Scan(&exists)

		if err != nil {
			t.Fatalf("Failed to check table existence: %v", err)
		}

		if !exists {
			t.Errorf("Expected table %q to exist after migration", tableName)
		}
	}
}

func TestApplyMigration_AlterTable(t *testing.T) {
	db, driver := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	tableName := "test_apply_migration_alter"

	// Create initial table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE `+tableName+` (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create initial table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

	// Apply migration to alter table
	migration := `ALTER TABLE ` + tableName + ` ADD COLUMN email TEXT NOT NULL DEFAULT 'user@example.com'`

	err = driver.ApplyMigration(ctx, db, migration)
	if err != nil {
		t.Fatalf("ApplyMigration failed: %v", err)
	}

	// Verify column was added
	columns, err := GetColumns(ctx, db, defaultSchema, tableName)
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 3 {
		t.Errorf("Expected 3 columns after ALTER, got %d", len(columns))
	}

	emailCol := findColumn(columns, "email")
	if emailCol == nil {
		t.Error("Expected to find 'email' column after ALTER")
	}
}

func TestApplyMigration_EmptyMigration(t *testing.T) {
	db, driver := getTestDb(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()

	// Apply empty migration (should succeed)
	err := driver.ApplyMigration(ctx, db, "")
	if err != nil {
		t.Errorf("Expected empty migration to succeed, got error: %v", err)
	}
}
