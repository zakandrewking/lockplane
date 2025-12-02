package generator

import (
	"strings"
	"testing"

	"github.com/lockplane/lockplane/internal/database"
)

func TestGenerateMigrationSQL_EnableRLS(t *testing.T) {
	current := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: false,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	desired := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	statements, err := GenerateMigrationSQL(current, desired)
	if err != nil {
		t.Fatalf("GenerateMigrationSQL failed: %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(statements))
	}

	expected := "ALTER TABLE users ENABLE ROW LEVEL SECURITY;"
	if statements[0] != expected {
		t.Errorf("Expected %q, got %q", expected, statements[0])
	}
}

func TestGenerateMigrationSQL_DisableRLS(t *testing.T) {
	current := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	desired := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: false,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	statements, err := GenerateMigrationSQL(current, desired)
	if err != nil {
		t.Fatalf("GenerateMigrationSQL failed: %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(statements))
	}

	expected := "ALTER TABLE users DISABLE ROW LEVEL SECURITY;"
	if statements[0] != expected {
		t.Errorf("Expected %q, got %q", expected, statements[0])
	}
}

func TestGenerateMigrationSQL_CreateTableWithRLS(t *testing.T) {
	current := &database.Schema{
		Tables: []database.Table{},
	}

	desired := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
					{Name: "email", Type: "text", Nullable: false},
				},
			},
		},
	}

	statements, err := GenerateMigrationSQL(current, desired)
	if err != nil {
		t.Fatalf("GenerateMigrationSQL failed: %v", err)
	}

	if len(statements) != 2 {
		t.Fatalf("Expected 2 statements (CREATE TABLE + ENABLE RLS), got %d", len(statements))
	}

	// Check CREATE TABLE statement
	if !strings.Contains(statements[0], "CREATE TABLE users") {
		t.Errorf("Expected CREATE TABLE statement, got %q", statements[0])
	}

	// Check ENABLE RLS statement
	expected := "ALTER TABLE users ENABLE ROW LEVEL SECURITY;"
	if statements[1] != expected {
		t.Errorf("Expected %q, got %q", expected, statements[1])
	}
}

func TestGenerateMigrationSQL_CreateTableWithoutRLS(t *testing.T) {
	current := &database.Schema{
		Tables: []database.Table{},
	}

	desired := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: false,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	statements, err := GenerateMigrationSQL(current, desired)
	if err != nil {
		t.Fatalf("GenerateMigrationSQL failed: %v", err)
	}

	// Should only have CREATE TABLE, no RLS statement
	if len(statements) != 1 {
		t.Fatalf("Expected 1 statement (CREATE TABLE only), got %d", len(statements))
	}

	if !strings.Contains(statements[0], "CREATE TABLE users") {
		t.Errorf("Expected CREATE TABLE statement, got %q", statements[0])
	}

	// Ensure no RLS statement was generated
	for _, stmt := range statements {
		if strings.Contains(stmt, "ROW LEVEL SECURITY") {
			t.Errorf("Expected no RLS statement, but found: %q", stmt)
		}
	}
}

func TestGenerateMigrationSQL_NoRLSChange(t *testing.T) {
	current := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	desired := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	statements, err := GenerateMigrationSQL(current, desired)
	if err != nil {
		t.Fatalf("GenerateMigrationSQL failed: %v", err)
	}

	// No changes should result in no statements
	if len(statements) != 0 {
		t.Errorf("Expected 0 statements, got %d: %v", len(statements), statements)
	}
}

func TestGenerateMigrationSQL_DropTable(t *testing.T) {
	current := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	desired := &database.Schema{
		Tables: []database.Table{},
	}

	statements, err := GenerateMigrationSQL(current, desired)
	if err != nil {
		t.Fatalf("GenerateMigrationSQL failed: %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(statements))
	}

	expected := "DROP TABLE users;"
	if statements[0] != expected {
		t.Errorf("Expected %q, got %q", expected, statements[0])
	}
}

func TestGenerateMigrationSQL_MultipleTables(t *testing.T) {
	current := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: false,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
			{
				Name:       "posts",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	desired := &database.Schema{
		Tables: []database.Table{
			{
				Name:       "users",
				RLSEnabled: true,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
			{
				Name:       "posts",
				RLSEnabled: false,
				Columns: []database.Column{
					{Name: "id", Type: "integer", IsPrimaryKey: true},
				},
			},
		},
	}

	statements, err := GenerateMigrationSQL(current, desired)
	if err != nil {
		t.Fatalf("GenerateMigrationSQL failed: %v", err)
	}

	if len(statements) != 2 {
		t.Fatalf("Expected 2 statements, got %d", len(statements))
	}

	// Check that both RLS changes are present (order may vary)
	statementsStr := strings.Join(statements, "\n")
	if !strings.Contains(statementsStr, "ALTER TABLE users ENABLE ROW LEVEL SECURITY") {
		t.Error("Expected ENABLE RLS statement for users table")
	}
	if !strings.Contains(statementsStr, "ALTER TABLE posts DISABLE ROW LEVEL SECURITY") {
		t.Error("Expected DISABLE RLS statement for posts table")
	}
}

func TestGenerateCreateTableSQL(t *testing.T) {
	defaultValue := "NOW()"
	table := &database.Table{
		Name: "users",
		Columns: []database.Column{
			{Name: "id", Type: "bigint", IsPrimaryKey: true, Nullable: false},
			{Name: "email", Type: "text", Nullable: false},
			{Name: "created_at", Type: "timestamp with time zone", Default: &defaultValue},
		},
	}

	sql, err := generateCreateTableSQL(table)
	if err != nil {
		t.Fatalf("generateCreateTableSQL failed: %v", err)
	}

	// Verify the SQL contains key elements
	if !strings.Contains(sql, "CREATE TABLE users") {
		t.Errorf("Expected CREATE TABLE users, got: %s", sql)
	}
	if !strings.Contains(sql, "id bigint PRIMARY KEY NOT NULL") {
		t.Errorf("Expected id column with PRIMARY KEY NOT NULL, got: %s", sql)
	}
	if !strings.Contains(sql, "email text NOT NULL") {
		t.Errorf("Expected email column with NOT NULL, got: %s", sql)
	}
	if !strings.Contains(sql, "DEFAULT NOW()") {
		t.Errorf("Expected created_at column with DEFAULT, got: %s", sql)
	}
}
