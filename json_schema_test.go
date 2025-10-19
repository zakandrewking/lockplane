package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateJSONSchema_Valid(t *testing.T) {
	path := filepath.Join("examples", "schemas-json", "simple.json")
	if err := ValidateJSONSchema(path); err != nil {
		t.Fatalf("Expected schema %s to be valid, got error: %v", path, err)
	}
}

func TestValidateJSONSchema_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"tablesz": []}`), 0o600); err != nil {
		t.Fatalf("Failed to write invalid schema file: %v", err)
	}

	if err := ValidateJSONSchema(invalidPath); err == nil {
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

	actual, err := LoadSchema(sqlPath)
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
						Type:         "pg_catalog.int8",
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
						Type:         "pg_catalog.int8",
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

	actual, err := LoadSchema(sqlPath)
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
						Type:         "pg_catalog.int8",
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
						Type:         "pg_catalog.varchar(100)",
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
	if bioCol.Type != "pg_catalog.varchar(100)" {
		t.Fatalf("expected bio type pg_catalog.varchar(100), got %s", bioCol.Type)
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

	nestedDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o700); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	file2 := filepath.Join(nestedDir, "010_alter_users.lp.sql")
	content2 := `
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
`
	if err := os.WriteFile(file2, []byte(content2), 0o600); err != nil {
		t.Fatalf("Failed to write %s: %v", file2, err)
	}

	actual, err := LoadSchema(tmpDir)
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
