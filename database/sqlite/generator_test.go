package sqlite

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

	// SQLite requires PRIMARY KEY before NOT NULL
	if !strings.Contains(sql, "id integer PRIMARY KEY") {
		t.Errorf("Expected SQL to contain id column definition with PRIMARY KEY, got: %s", sql)
	}

	if !strings.Contains(sql, "email text NOT NULL") {
		t.Errorf("Expected SQL to contain email column definition, got: %s", sql)
	}
}

func TestGenerator_DropTable(t *testing.T) {
	gen := NewGenerator()

	table := database.Table{Name: "old_table"}
	sql, desc := gen.DropTable(table)

	// SQLite doesn't support CASCADE
	if sql != "DROP TABLE old_table" {
		t.Errorf("Expected 'DROP TABLE old_table' (no CASCADE), got: %s", sql)
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

func TestGenerator_ModifyColumn(t *testing.T) {
	gen := NewGenerator()

	diff := database.ColumnDiff{
		ColumnName: "age",
		Old:        database.Column{Name: "age", Type: "integer", Nullable: true},
		New:        database.Column{Name: "age", Type: "bigint", Nullable: true},
		Changes:    []string{"type"},
	}

	steps := gen.ModifyColumn("users", diff)

	// SQLite doesn't support ALTER COLUMN, should return warning step
	if len(steps) != 1 {
		t.Fatalf("Expected 1 warning step, got %d", len(steps))
	}

	if !strings.Contains(steps[0].Description, "SQLite limitation") {
		t.Errorf("Expected limitation warning in description, got: %s", steps[0].Description)
	}

	if !strings.Contains(steps[0].SQL, "--") {
		t.Errorf("Expected comment SQL, got: %s", steps[0].SQL)
	}
}

func TestGenerator_ModifyColumn_NoChanges(t *testing.T) {
	gen := NewGenerator()

	diff := database.ColumnDiff{
		ColumnName: "age",
		Old:        database.Column{Name: "age", Type: "integer", Nullable: true},
		New:        database.Column{Name: "age", Type: "integer", Nullable: true},
		Changes:    []string{},
	}

	steps := gen.ModifyColumn("users", diff)

	// No changes should result in no steps
	if len(steps) != 0 {
		t.Errorf("Expected 0 steps for no changes, got %d", len(steps))
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

func TestGenerator_CreateTable_WithForeignKeys(t *testing.T) {
	gen := NewGenerator()

	onDelete := "CASCADE"
	table := database.Table{
		Name: "posts",
		Columns: []database.Column{
			{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
			{Name: "user_id", Type: "integer", Nullable: false},
			{Name: "title", Type: "text", Nullable: false},
		},
		ForeignKeys: []database.ForeignKey{
			{
				Name:              "fk_posts_user_id",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
				OnDelete:          &onDelete,
			},
		},
	}

	sql, desc := gen.CreateTable(table)

	// Verify description
	if !strings.Contains(desc, "Create table posts") {
		t.Errorf("Expected description to contain 'Create table posts', got: %s", desc)
	}

	// Verify SQL contains foreign key constraint
	if !strings.Contains(sql, "CONSTRAINT fk_posts_user_id") {
		t.Errorf("Expected SQL to contain foreign key constraint, got: %s", sql)
	}

	if !strings.Contains(sql, "FOREIGN KEY (user_id)") {
		t.Errorf("Expected SQL to contain FOREIGN KEY definition, got: %s", sql)
	}

	if !strings.Contains(sql, "REFERENCES users (id)") {
		t.Errorf("Expected SQL to contain REFERENCES clause, got: %s", sql)
	}

	if !strings.Contains(sql, "ON DELETE CASCADE") {
		t.Errorf("Expected SQL to contain ON DELETE CASCADE, got: %s", sql)
	}
}

func TestGenerator_FormatForeignKeyConstraint(t *testing.T) {
	gen := NewGenerator()

	onDelete := "CASCADE"
	onUpdate := "RESTRICT"

	tests := []struct {
		name     string
		fk       database.ForeignKey
		expected []string
	}{
		{
			name: "simple foreign key",
			fk: database.ForeignKey{
				Name:              "fk_posts_user",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
			},
			expected: []string{
				"CONSTRAINT fk_posts_user",
				"FOREIGN KEY (user_id)",
				"REFERENCES users (id)",
			},
		},
		{
			name: "foreign key with ON DELETE",
			fk: database.ForeignKey{
				Name:              "fk_posts_user",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
				OnDelete:          &onDelete,
			},
			expected: []string{
				"CONSTRAINT fk_posts_user",
				"FOREIGN KEY (user_id)",
				"REFERENCES users (id)",
				"ON DELETE CASCADE",
			},
		},
		{
			name: "foreign key with ON UPDATE",
			fk: database.ForeignKey{
				Name:              "fk_posts_user",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
				OnUpdate:          &onUpdate,
			},
			expected: []string{
				"CONSTRAINT fk_posts_user",
				"FOREIGN KEY (user_id)",
				"REFERENCES users (id)",
				"ON UPDATE RESTRICT",
			},
		},
		{
			name: "composite foreign key",
			fk: database.ForeignKey{
				Name:              "fk_order_items",
				Columns:           []string{"order_id", "product_id"},
				ReferencedTable:   "orders",
				ReferencedColumns: []string{"id", "product_id"},
			},
			expected: []string{
				"CONSTRAINT fk_order_items",
				"FOREIGN KEY (order_id, product_id)",
				"REFERENCES orders (id, product_id)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.FormatForeignKeyConstraint(tt.fk)
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected result to contain '%s', got: %s", exp, result)
				}
			}
		})
	}
}

func TestGenerator_RecreateTableWithForeignKey(t *testing.T) {
	gen := NewGenerator()

	table := database.Table{
		Name: "posts",
		Columns: []database.Column{
			{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
			{Name: "title", Type: "text", Nullable: false},
			{Name: "user_id", Type: "integer", Nullable: false},
		},
		ForeignKeys: []database.ForeignKey{},
	}

	newFK := database.ForeignKey{
		Name:              "fk_posts_user_id",
		Columns:           []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	}

	steps := gen.RecreateTableWithForeignKey(table, newFK)

	// Should have 4 steps: create temp table, copy data, drop old table, rename
	if len(steps) != 4 {
		t.Fatalf("Expected 4 steps, got %d", len(steps))
	}

	// Step 1: Create new table with foreign key
	if !strings.Contains(steps[0].SQL, "CREATE TABLE posts_new") {
		t.Errorf("Expected step 1 to create posts_new, got: %s", steps[0].SQL)
	}
	if !strings.Contains(steps[0].SQL, "CONSTRAINT fk_posts_user_id") {
		t.Errorf("Expected step 1 to include foreign key, got: %s", steps[0].SQL)
	}

	// Step 2: Copy data
	if !strings.Contains(steps[1].SQL, "INSERT INTO posts_new") {
		t.Errorf("Expected step 2 to insert data, got: %s", steps[1].SQL)
	}
	if !strings.Contains(steps[1].SQL, "SELECT id, title, user_id FROM posts") {
		t.Errorf("Expected step 2 to select all columns, got: %s", steps[1].SQL)
	}

	// Step 3: Drop old table
	if steps[2].SQL != "DROP TABLE posts" {
		t.Errorf("Expected step 3 to drop posts, got: %s", steps[2].SQL)
	}

	// Step 4: Rename new table
	if steps[3].SQL != "ALTER TABLE posts_new RENAME TO posts" {
		t.Errorf("Expected step 4 to rename table, got: %s", steps[3].SQL)
	}
}

func TestGenerator_RecreateTableWithoutForeignKey(t *testing.T) {
	gen := NewGenerator()

	onDelete := "CASCADE"
	table := database.Table{
		Name: "posts",
		Columns: []database.Column{
			{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
			{Name: "title", Type: "text", Nullable: false},
			{Name: "user_id", Type: "integer", Nullable: false},
		},
		ForeignKeys: []database.ForeignKey{
			{
				Name:              "fk_posts_user_id",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
				OnDelete:          &onDelete,
			},
		},
	}

	steps := gen.RecreateTableWithoutForeignKey(table, "fk_posts_user_id")

	// Should have 4 steps: create temp table, copy data, drop old table, rename
	if len(steps) != 4 {
		t.Fatalf("Expected 4 steps, got %d", len(steps))
	}

	// Step 1: Create new table without the foreign key
	if !strings.Contains(steps[0].SQL, "CREATE TABLE posts_new") {
		t.Errorf("Expected step 1 to create posts_new, got: %s", steps[0].SQL)
	}
	if strings.Contains(steps[0].SQL, "fk_posts_user_id") {
		t.Errorf("Expected step 1 to NOT include the foreign key, got: %s", steps[0].SQL)
	}

	// Step 2: Copy data
	if !strings.Contains(steps[1].SQL, "INSERT INTO posts_new") {
		t.Errorf("Expected step 2 to insert data, got: %s", steps[1].SQL)
	}

	// Step 3: Drop old table
	if steps[2].SQL != "DROP TABLE posts" {
		t.Errorf("Expected step 3 to drop posts, got: %s", steps[2].SQL)
	}

	// Step 4: Rename new table
	if steps[3].SQL != "ALTER TABLE posts_new RENAME TO posts" {
		t.Errorf("Expected step 4 to rename table, got: %s", steps[3].SQL)
	}
}

func TestGenerator_FormatColumnDefinition(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name     string
		column   database.Column
		expected []string // Parts that should be in the output
		notIn    []string // Parts that should NOT be in the output
	}{
		{
			name: "simple column",
			column: database.Column{
				Name:     "name",
				Type:     "text",
				Nullable: true,
			},
			expected: []string{"name text"},
			notIn:    []string{"NOT NULL", "PRIMARY KEY"},
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
			// SQLite requires PRIMARY KEY before NOT NULL
			expected: []string{"id integer", "PRIMARY KEY", "NOT NULL"},
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
			for _, notExp := range tt.notIn {
				if strings.Contains(result, notExp) {
					t.Errorf("Expected result to NOT contain '%s', got: %s", notExp, result)
				}
			}
		})
	}
}

func TestGenerator_FormatColumnDefinition_PrimaryKeyOrder(t *testing.T) {
	gen := NewGenerator()

	col := database.Column{
		Name:         "id",
		Type:         "integer",
		Nullable:     false,
		IsPrimaryKey: true,
	}

	result := gen.FormatColumnDefinition(col)

	// In SQLite, PRIMARY KEY should come before NOT NULL
	pkIndex := strings.Index(result, "PRIMARY KEY")
	notNullIndex := strings.Index(result, "NOT NULL")

	if pkIndex == -1 {
		t.Error("Expected PRIMARY KEY in result")
	}

	if notNullIndex == -1 {
		t.Error("Expected NOT NULL in result")
	}

	if pkIndex > notNullIndex {
		t.Errorf("PRIMARY KEY should come before NOT NULL in SQLite, got: %s", result)
	}
}

func TestGenerator_ParameterPlaceholder(t *testing.T) {
	gen := NewGenerator()

	// SQLite uses ? for all positions
	tests := []struct {
		position int
		expected string
	}{
		{1, "?"},
		{2, "?"},
		{10, "?"},
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
