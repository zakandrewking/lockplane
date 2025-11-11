package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/schema"
)

func TestValidateJSONSchema_Valid(t *testing.T) {
	path := filepath.Join("examples", "schemas-json", "simple.json")
	if err := schema.ValidateJSONSchema(path); err != nil {
		t.Fatalf("Expected schema %s to be valid, got error: %v", path, err)
	}
}

func TestValidateJSONSchema_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"tablesz": []}`), 0o600); err != nil {
		t.Fatalf("Failed to write invalid schema file: %v", err)
	}

	if err := schema.ValidateJSONSchema(invalidPath); err == nil {
		t.Fatalf("Expected schema %s to be invalid", invalidPath)
	}
}

func TestLoadSchemaFromLPSQL(t *testing.T) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "schema.lp.sql")

	sqlDDL := `
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    email TEXT NOT NULL,
    team_id BIGINT,
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_team_fk FOREIGN KEY (team_id) REFERENCES teams(id)
);
`
	if err := os.WriteFile(sqlPath, []byte(sqlDDL), 0o600); err != nil {
		t.Fatalf("Failed to write SQL fixture: %v", err)
	}

	actual, err := schema.LoadSchema(sqlPath)
	if err != nil {
		t.Fatalf("LoadSchema returned error: %v", err)
	}

	expected := &Schema{
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{
						Name:         "id",
						Type:         "bigint",
						Nullable:     false,
						IsPrimaryKey: true,
					},
					{
						Name:         "email",
						Type:         "text",
						Nullable:     false,
						IsPrimaryKey: false,
					},
					{
						Name:         "team_id",
						Type:         "bigint",
						Nullable:     true,
						IsPrimaryKey: false,
					},
				},
				Indexes: []Index{
					{
						Name:    "users_email_key",
						Columns: []string{"email"},
						Unique:  true,
					},
				},
				ForeignKeys: []ForeignKey{
					{
						Name:              "users_team_fk",
						Columns:           []string{"team_id"},
						ReferencedTable:   "teams",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}

	compareSchemas(t, expected, actual)

	var usersTable *Table
	for i := range actual.Tables {
		if actual.Tables[i].Name == "users" {
			usersTable = &actual.Tables[i]
			break
		}
	}
	if usersTable == nil {
		t.Fatalf("expected users table in parsed schema")
		return
	}
	if len(usersTable.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(usersTable.ForeignKeys))
	}
	fk := usersTable.ForeignKeys[0]
	if fk.ReferencedTable != "teams" {
		t.Fatalf("expected foreign key to reference teams, got %s", fk.ReferencedTable)
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "team_id" {
		t.Fatalf("expected foreign key column team_id, got %v", fk.Columns)
	}
	if len(fk.ReferencedColumns) != 1 || fk.ReferencedColumns[0] != "id" {
		t.Fatalf("expected referenced column id, got %v", fk.ReferencedColumns)
	}
}

func TestLoadSchemaSQLitePreservesTypes(t *testing.T) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "sqlite_schema.lp.sql")

	sqlDDL := `
CREATE TABLE todos (
    id INTEGER PRIMARY KEY,
    completed INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`

	if err := os.WriteFile(sqlPath, []byte(sqlDDL), 0o600); err != nil {
		t.Fatalf("Failed to write SQL fixture: %v", err)
	}

	loadedSchema, err := schema.LoadSchemaWithOptions(sqlPath, &schema.SchemaLoadOptions{Dialect: database.DialectSQLite})
	if err != nil {
		t.Fatalf("LoadSchemaWithOptions returned error: %v", err)
	}

	if loadedSchema.Dialect != database.DialectSQLite {
		t.Fatalf("expected schema dialect sqlite, got %s", loadedSchema.Dialect)
	}

	if len(loadedSchema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(loadedSchema.Tables))
	}

	todos := &loadedSchema.Tables[0]
	completed := findColumnByName(t, todos, "completed")
	if completed.Type != "INTEGER" {
		t.Fatalf("expected completed type INTEGER, got %s", completed.Type)
	}
	if completed.LogicalType() != "integer" {
		t.Fatalf("expected logical type integer, got %s", completed.LogicalType())
	}

	createdAt := findColumnByName(t, todos, "created_at")
	if createdAt.Default == nil {
		t.Fatalf("expected created_at default to be set")
	}
	lowerDefault := strings.ToLower(*createdAt.Default)
	if !strings.Contains(lowerDefault, "datetime('now'") {
		t.Fatalf("expected created_at default to contain datetime('now'), got %s", *createdAt.Default)
	}
	if createdAt.DefaultMetadata == nil {
		t.Fatalf("expected default metadata to be populated")
	}
	if !strings.Contains(strings.ToLower(createdAt.DefaultMetadata.Raw), "datetime('now'") {
		t.Fatalf("expected default metadata raw to contain datetime('now'), got %s", createdAt.DefaultMetadata.Raw)
	}
}

func TestLoadSchemaFromLPSQLWithAlterStatements(t *testing.T) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "schema.lp.sql")

	sqlDDL := `
CREATE TABLE users (
    id BIGINT,
    email TEXT
);
ALTER TABLE users ADD COLUMN bio TEXT;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ALTER COLUMN bio TYPE VARCHAR(100);
ALTER TABLE users ALTER COLUMN email SET DEFAULT 'n/a';
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
`
	if err := os.WriteFile(sqlPath, []byte(sqlDDL), 0o600); err != nil {
		t.Fatalf("Failed to write SQL fixture: %v", err)
	}

	actual, err := schema.LoadSchema(sqlPath)
	if err != nil {
		t.Fatalf("LoadSchema returned error: %v", err)
	}

	expected := &Schema{
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{
						Name:         "id",
						Type:         "bigint",
						Nullable:     true,
						IsPrimaryKey: false,
					},
					{
						Name:         "email",
						Type:         "text",
						Nullable:     false,
						IsPrimaryKey: false,
						Default:      strPtr("'n/a'"),
					},
					{
						Name:         "bio",
						Type:         "varchar(100)",
						Nullable:     true,
						IsPrimaryKey: false,
					},
				},
				Indexes: []Index{
					{
						Name:    "users_email_key",
						Columns: []string{"email"},
						Unique:  true,
					},
				},
			},
		},
	}

	compareSchemas(t, expected, actual)

	var usersTable *Table
	for i := range actual.Tables {
		if actual.Tables[i].Name == "users" {
			usersTable = &actual.Tables[i]
			break
		}
	}
	if usersTable == nil {
		t.Fatalf("expected users table in parsed schema")
	}

	emailCol := findColumnByName(t, usersTable, "email")
	if emailCol.Default == nil || *emailCol.Default != "'n/a'" {
		t.Fatalf("expected email default 'n/a', got %v", emailCol.Default)
	}

	bioCol := findColumnByName(t, usersTable, "bio")
	if bioCol.Type != "varchar(100)" {
		t.Fatalf("expected bio type varchar(100), got %s", bioCol.Type)
	}
}

func TestLoadSchemaFromLPSQLDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "001_create_users.lp.sql")
	content1 := `
CREATE TABLE users (
    id BIGINT,
    email TEXT
);
`
	if err := os.WriteFile(file1, []byte(content1), 0o600); err != nil {
		t.Fatalf("Failed to write %s: %v", file1, err)
	}

	file2 := filepath.Join(tmpDir, "010_alter_users.lp.sql")
	content2 := `
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
`
	if err := os.WriteFile(file2, []byte(content2), 0o600); err != nil {
		t.Fatalf("Failed to write %s: %v", file2, err)
	}

	actual, err := schema.LoadSchema(tmpDir)
	if err != nil {
		t.Fatalf("LoadSchema returned error: %v", err)
	}

	if len(actual.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(actual.Tables))
	}

	usersTable := &actual.Tables[0]

	emailCol := findColumnByName(t, usersTable, "email")
	if emailCol.Nullable {
		t.Fatalf("expected email to be NOT NULL after directory load")
	}

	if len(usersTable.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(usersTable.Indexes))
	}
	if usersTable.Indexes[0].Name != "users_email_key" {
		t.Fatalf("expected index name users_email_key, got %s", usersTable.Indexes[0].Name)
	}
	if len(usersTable.Indexes[0].Columns) != 1 || usersTable.Indexes[0].Columns[0] != "email" {
		t.Fatalf("expected users_email_key to cover email, got %v", usersTable.Indexes[0].Columns)
	}
}

func findColumnByName(t *testing.T, table *Table, name string) *Column {
	t.Helper()
	for i := range table.Columns {
		if table.Columns[i].Name == name {
			return &table.Columns[i]
		}
	}
	t.Fatalf("column %s not found in table %s", name, table.Name)
	return nil
}

func strPtr(s string) *string {
	return &s
}

func TestIsConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Postgres connection strings
		{
			name:     "postgres URL",
			input:    "postgres://user:pass@localhost:5432/dbname",
			expected: true,
		},
		{
			name:     "postgresql URL",
			input:    "postgresql://user:pass@localhost:5432/dbname?sslmode=disable",
			expected: true,
		},
		{
			name:     "postgres URL uppercase",
			input:    "POSTGRES://USER:PASS@LOCALHOST:5432/DBNAME",
			expected: true,
		},
		// SQLite connection strings
		{
			name:     "sqlite URL",
			input:    "sqlite://path/to/database.db",
			expected: true,
		},
		{
			name:     "file URL",
			input:    "file:path/to/database.db",
			expected: true,
		},
		{
			name:     "in-memory sqlite",
			input:    ":memory:",
			expected: true,
		},
		// Turso/libSQL connection strings
		{
			name:     "libsql URL",
			input:    "libsql://mydb-user.turso.io",
			expected: true,
		},
		{
			name:     "libsql URL with auth token",
			input:    "libsql://mydb-user.turso.io?authToken=eyJhbGc...",
			expected: true,
		},
		{
			name:     "libsql URL uppercase",
			input:    "LIBSQL://MYDB-USER.TURSO.IO",
			expected: true,
		},
		// File paths that should NOT be connection strings
		{
			name:     "JSON file",
			input:    "schema.json",
			expected: false,
		},
		{
			name:     "SQL file",
			input:    "schema.lp.sql",
			expected: false,
		},
		{
			name:     "directory path",
			input:    "schema/",
			expected: false,
		},
		{
			name:     "absolute JSON path",
			input:    "/path/to/schema.json",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionString(tt.input)
			if result != tt.expected {
				t.Errorf("isConnectionString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsConnectionString_ExistingDBFile(t *testing.T) {
	// Create a temporary SQLite file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := os.WriteFile(dbPath, []byte{}, 0o600); err != nil {
		t.Fatalf("Failed to create test.db: %v", err)
	}

	// .db files should always be treated as SQLite databases (connection strings)
	// regardless of whether they exist on disk
	result := isConnectionString(dbPath)
	if !result {
		t.Errorf("isConnectionString(%q) = false, want true (.db files should be treated as SQLite databases)", dbPath)
	}
}

func TestLoadSchemaOrIntrospect_FilePath(t *testing.T) {
	// Test that LoadSchemaOrIntrospect works with file paths (backward compatibility)
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.lp.sql")

	sqlDDL := `
CREATE TABLE test_table (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL
);
`
	if err := os.WriteFile(schemaPath, []byte(sqlDDL), 0o600); err != nil {
		t.Fatalf("Failed to write SQL fixture: %v", err)
	}

	schema, err := LoadSchemaOrIntrospect(schemaPath)
	if err != nil {
		t.Fatalf("LoadSchemaOrIntrospect returned error: %v", err)
	}

	if len(schema.Tables) != 1 {
		t.Fatalf("Expected 1 table, got %d", len(schema.Tables))
	}

	if schema.Tables[0].Name != "test_table" {
		t.Errorf("Expected table name 'test_table', got %q", schema.Tables[0].Name)
	}
}
