package postgres

import (
	"strings"
	"testing"

	"github.com/lockplane/lockplane/database"
)

func TestGenerator_CreateTable(t *testing.T) {
	gen := NewGenerator()

	table := database.Table{
		Name: "users",
		Columns: []database.Column{
			{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
			{Name: "email", Type: "text", Nullable: false},
			{Name: "age", Type: "integer", Nullable: true},
		},
	}

	sql, desc := gen.CreateTable(table)

	// Verify description
	if !strings.Contains(desc, "Create table users") {
		t.Errorf("Expected description to contain 'Create table users', got: %s", desc)
	}

	// Verify SQL
	if !strings.Contains(sql, "CREATE TABLE users") {
		t.Errorf("Expected SQL to contain 'CREATE TABLE users', got: %s", sql)
	}

	if !strings.Contains(sql, "id integer NOT NULL PRIMARY KEY") {
		t.Errorf("Expected SQL to contain id column definition, got: %s", sql)
	}

	if !strings.Contains(sql, "email text NOT NULL") {
		t.Errorf("Expected SQL to contain email column definition, got: %s", sql)
	}

	if !strings.Contains(sql, "age integer") && strings.Contains(sql, "age") {
		// Should have age without NOT NULL
		if strings.Contains(sql, "age integer NOT NULL") {
			t.Errorf("Expected age to be nullable, got: %s", sql)
		}
	}
}

func TestGenerator_DropTable(t *testing.T) {
	gen := NewGenerator()

	table := database.Table{Name: "old_table"}
	sql, desc := gen.DropTable(table)

	if sql != "DROP TABLE old_table CASCADE" {
		t.Errorf("Expected 'DROP TABLE old_table CASCADE', got: %s", sql)
	}

	if !strings.Contains(desc, "Drop table old_table") {
		t.Errorf("Expected description to contain 'Drop table old_table', got: %s", desc)
	}
}

func TestGenerator_AddColumn(t *testing.T) {
	gen := NewGenerator()

	col := database.Column{
		Name:     "phone",
		Type:     "text",
		Nullable: true,
	}

	sql, desc := gen.AddColumn("users", col)

	if !strings.Contains(sql, "ALTER TABLE users ADD COLUMN phone text") {
		t.Errorf("Expected ALTER TABLE ADD COLUMN, got: %s", sql)
	}

	if strings.Contains(sql, "NOT NULL") {
		t.Errorf("Expected nullable column (no NOT NULL), got: %s", sql)
	}

	if !strings.Contains(desc, "Add column phone to table users") {
		t.Errorf("Expected appropriate description, got: %s", desc)
	}
}

func TestGenerator_AddColumnWithDefault(t *testing.T) {
	gen := NewGenerator()

	defaultVal := "0"
	col := database.Column{
		Name:     "score",
		Type:     "integer",
		Nullable: false,
		Default:  &defaultVal,
	}

	sql, _ := gen.AddColumn("users", col)

	if !strings.Contains(sql, "DEFAULT 0") {
		t.Errorf("Expected default value in SQL, got: %s", sql)
	}

	if !strings.Contains(sql, "NOT NULL") {
		t.Errorf("Expected NOT NULL in SQL, got: %s", sql)
	}
}

func TestGenerator_DropColumn(t *testing.T) {
	gen := NewGenerator()

	col := database.Column{Name: "deprecated_field"}
	sql, desc := gen.DropColumn("users", col)

	if sql != "ALTER TABLE users DROP COLUMN deprecated_field" {
		t.Errorf("Expected 'ALTER TABLE users DROP COLUMN deprecated_field', got: %s", sql)
	}

	if !strings.Contains(desc, "Drop column deprecated_field from table users") {
		t.Errorf("Expected appropriate description, got: %s", desc)
	}
}

func TestGenerator_ModifyColumn_Type(t *testing.T) {
	gen := NewGenerator()

	diff := database.ColumnDiff{
		ColumnName: "age",
		Old:        database.Column{Name: "age", Type: "integer", Nullable: true},
		New:        database.Column{Name: "age", Type: "bigint", Nullable: true},
		Changes:    []string{"type"},
	}

	steps := gen.ModifyColumn("users", diff)

	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	if steps[0].SQL != "ALTER TABLE users ALTER COLUMN age TYPE bigint" {
		t.Errorf("Expected type change SQL, got: %s", steps[0].SQL)
	}
}

func TestGenerator_ModifyColumn_Nullable(t *testing.T) {
	gen := NewGenerator()

	// Test setting NOT NULL
	diff := database.ColumnDiff{
		ColumnName: "email",
		Old:        database.Column{Name: "email", Type: "text", Nullable: true},
		New:        database.Column{Name: "email", Type: "text", Nullable: false},
		Changes:    []string{"nullable"},
	}

	steps := gen.ModifyColumn("users", diff)
	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	if steps[0].SQL != "ALTER TABLE users ALTER COLUMN email SET NOT NULL" {
		t.Errorf("Expected SET NOT NULL, got: %s", steps[0].SQL)
	}

	// Test removing NOT NULL
	diff.Old.Nullable = false
	diff.New.Nullable = true

	steps = gen.ModifyColumn("users", diff)
	if steps[0].SQL != "ALTER TABLE users ALTER COLUMN email DROP NOT NULL" {
		t.Errorf("Expected DROP NOT NULL, got: %s", steps[0].SQL)
	}
}

func TestGenerator_ModifyColumn_Default(t *testing.T) {
	gen := NewGenerator()

	defaultVal := "now()"
	diff := database.ColumnDiff{
		ColumnName: "created_at",
		Old:        database.Column{Name: "created_at", Type: "timestamp", Nullable: true},
		New:        database.Column{Name: "created_at", Type: "timestamp", Nullable: true, Default: &defaultVal},
		Changes:    []string{"default"},
	}

	steps := gen.ModifyColumn("users", diff)
	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	if steps[0].SQL != "ALTER TABLE users ALTER COLUMN created_at SET DEFAULT now()" {
		t.Errorf("Expected SET DEFAULT, got: %s", steps[0].SQL)
	}

	// Test removing default
	diff.Old.Default = &defaultVal
	diff.New.Default = nil

	steps = gen.ModifyColumn("users", diff)
	if steps[0].SQL != "ALTER TABLE users ALTER COLUMN created_at DROP DEFAULT" {
		t.Errorf("Expected DROP DEFAULT, got: %s", steps[0].SQL)
	}
}

func TestGenerator_ModifyColumn_Multiple(t *testing.T) {
	gen := NewGenerator()

	defaultVal := "0"
	diff := database.ColumnDiff{
		ColumnName: "score",
		Old:        database.Column{Name: "score", Type: "integer", Nullable: true},
		New:        database.Column{Name: "score", Type: "bigint", Nullable: false, Default: &defaultVal},
		Changes:    []string{"type", "nullable", "default"},
	}

	steps := gen.ModifyColumn("users", diff)

	// Should generate 3 steps for type, nullable, and default changes
	if len(steps) != 3 {
		t.Fatalf("Expected 3 steps, got %d", len(steps))
	}
}

func TestGenerator_AddIndex(t *testing.T) {
	gen := NewGenerator()

	idx := database.Index{
		Name:    "idx_users_email",
		Columns: []string{"email"},
		Unique:  true,
	}

	sql, desc := gen.AddIndex("users", idx)

	if sql != "CREATE UNIQUE INDEX idx_users_email ON users (email)" {
		t.Errorf("Expected CREATE UNIQUE INDEX, got: %s", sql)
	}

	if !strings.Contains(desc, "Create index idx_users_email on table users") {
		t.Errorf("Expected appropriate description, got: %s", desc)
	}
}

func TestGenerator_AddIndex_MultiColumn(t *testing.T) {
	gen := NewGenerator()

	idx := database.Index{
		Name:    "idx_users_name_email",
		Columns: []string{"name", "email"},
		Unique:  false,
	}

	sql, _ := gen.AddIndex("users", idx)

	if sql != "CREATE INDEX idx_users_name_email ON users (name, email)" {
		t.Errorf("Expected multi-column index, got: %s", sql)
	}
}

func TestGenerator_DropIndex(t *testing.T) {
	gen := NewGenerator()

	idx := database.Index{Name: "idx_old"}
	sql, desc := gen.DropIndex("users", idx)

	if sql != "DROP INDEX idx_old" {
		t.Errorf("Expected 'DROP INDEX idx_old', got: %s", sql)
	}

	if !strings.Contains(desc, "Drop index idx_old from table users") {
		t.Errorf("Expected appropriate description, got: %s", desc)
	}
}

func TestGenerator_AddForeignKey(t *testing.T) {
	gen := NewGenerator()

	fk := database.ForeignKey{
		Name:              "fk_posts_user_id",
		Columns:           []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	}

	sql, desc := gen.AddForeignKey("posts", fk)

	expected := "ALTER TABLE posts ADD CONSTRAINT fk_posts_user_id FOREIGN KEY (user_id) REFERENCES users (id)"
	if sql != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, sql)
	}

	if !strings.Contains(desc, "Add foreign key fk_posts_user_id to table posts") {
		t.Errorf("Expected appropriate description, got: %s", desc)
	}
}

func TestGenerator_AddForeignKey_WithActions(t *testing.T) {
	gen := NewGenerator()

	onDelete := "CASCADE"
	onUpdate := "RESTRICT"
	fk := database.ForeignKey{
		Name:              "fk_posts_user_id",
		Columns:           []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
		OnDelete:          &onDelete,
		OnUpdate:          &onUpdate,
	}

	sql, _ := gen.AddForeignKey("posts", fk)

	if !strings.Contains(sql, "ON DELETE CASCADE") {
		t.Errorf("Expected ON DELETE CASCADE, got: %s", sql)
	}

	if !strings.Contains(sql, "ON UPDATE RESTRICT") {
		t.Errorf("Expected ON UPDATE RESTRICT, got: %s", sql)
	}
}

func TestGenerator_DropForeignKey(t *testing.T) {
	gen := NewGenerator()

	fk := database.ForeignKey{Name: "fk_posts_user_id"}
	sql, desc := gen.DropForeignKey("posts", fk)

	if sql != "ALTER TABLE posts DROP CONSTRAINT fk_posts_user_id" {
		t.Errorf("Expected 'ALTER TABLE posts DROP CONSTRAINT fk_posts_user_id', got: %s", sql)
	}

	if !strings.Contains(desc, "Drop foreign key fk_posts_user_id from table posts") {
		t.Errorf("Expected appropriate description, got: %s", desc)
	}
}

func TestGenerator_FormatColumnDefinition(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name     string
		column   database.Column
		expected []string // Parts that should be in the output
	}{
		{
			name: "simple column",
			column: database.Column{
				Name:     "name",
				Type:     "text",
				Nullable: true,
			},
			expected: []string{"name text"},
		},
		{
			name: "not null column",
			column: database.Column{
				Name:     "email",
				Type:     "text",
				Nullable: false,
			},
			expected: []string{"email text", "NOT NULL"},
		},
		{
			name: "column with default",
			column: database.Column{
				Name:     "age",
				Type:     "integer",
				Nullable: true,
				Default:  ptrString("0"),
			},
			expected: []string{"age integer", "DEFAULT 0"},
		},
		{
			name: "primary key column",
			column: database.Column{
				Name:         "id",
				Type:         "integer",
				Nullable:     false,
				IsPrimaryKey: true,
			},
			expected: []string{"id integer", "NOT NULL", "PRIMARY KEY"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.FormatColumnDefinition(tt.column)
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected result to contain '%s', got: %s", exp, result)
				}
			}
		})
	}
}

func TestGenerator_ParameterPlaceholder(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		position int
		expected string
	}{
		{1, "$1"},
		{2, "$2"},
		{10, "$10"},
	}

	for _, tt := range tests {
		result := gen.ParameterPlaceholder(tt.position)
		if result != tt.expected {
			t.Errorf("ParameterPlaceholder(%d) = %s, want %s", tt.position, result, tt.expected)
		}
	}
}

// Helper function
func ptrString(s string) *string {
	return &s
}
