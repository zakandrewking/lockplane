package sqlite

import (
	"testing"

	"github.com/lockplane/lockplane/database"
)

func TestNewDriver(t *testing.T) {
	driver := NewDriver()

	if driver == nil {
		t.Fatal("Expected non-nil driver")
	}

	if driver.Introspector == nil {
		t.Error("Expected non-nil introspector")
	}

	if driver.Generator == nil {
		t.Error("Expected non-nil generator")
	}
}

func TestDriver_Name(t *testing.T) {
	driver := NewDriver()

	if driver.Name() != "sqlite" {
		t.Errorf("Expected name 'sqlite', got '%s'", driver.Name())
	}
}

func TestDriver_SupportsFeature(t *testing.T) {
	driver := NewDriver()

	tests := []struct {
		feature  string
		expected bool
	}{
		{"CASCADE", false},               // SQLite doesn't support CASCADE on DROP TABLE
		{"ALTER_COLUMN_TYPE", false},     // Would require table recreation
		{"ALTER_COLUMN_NULLABLE", false}, // Would require table recreation
		{"ALTER_COLUMN_DEFAULT", false},  // Would require table recreation
		{"ALTER_ADD_FOREIGN_KEY", false}, // Foreign keys must be defined at table creation
		{"FOREIGN_KEYS", true},           // Supports foreign keys at table creation
		{"DROP_COLUMN", true},            // SQLite 3.35.0+
		{"UNSUPPORTED_FEATURE", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			result := driver.SupportsFeature(tt.feature)
			if result != tt.expected {
				t.Errorf("SupportsFeature(%s) = %v, want %v", tt.feature, result, tt.expected)
			}
		})
	}
}

func TestDriver_ImplementsInterface(t *testing.T) {
	var _ database.Driver = (*Driver)(nil)
	var _ database.Introspector = (*Introspector)(nil)
	var _ database.SQLGenerator = (*Generator)(nil)
}

func TestDriver_CreateTable(t *testing.T) {
	driver := NewDriver()

	table := database.Table{
		Name: "test_table",
		Columns: []database.Column{
			{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
		},
	}

	sql, desc := driver.CreateTable(table)

	if sql == "" {
		t.Error("Expected non-empty SQL")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_DropTable(t *testing.T) {
	driver := NewDriver()

	table := database.Table{Name: "test_table"}
	sql, desc := driver.DropTable(table)

	if sql == "" {
		t.Error("Expected non-empty SQL")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_AddColumn(t *testing.T) {
	driver := NewDriver()

	col := database.Column{Name: "new_col", Type: "text", Nullable: true}
	sql, desc := driver.AddColumn("test_table", col)

	if sql == "" {
		t.Error("Expected non-empty SQL")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_DropColumn(t *testing.T) {
	driver := NewDriver()

	col := database.Column{Name: "old_col"}
	sql, desc := driver.DropColumn("test_table", col)

	if sql == "" {
		t.Error("Expected non-empty SQL")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_ModifyColumn(t *testing.T) {
	driver := NewDriver()

	diff := database.ColumnDiff{
		ColumnName: "test_col",
		Old:        database.Column{Name: "test_col", Type: "integer", Nullable: true},
		New:        database.Column{Name: "test_col", Type: "bigint", Nullable: true},
		Changes:    []string{"type"},
	}

	steps := driver.ModifyColumn("test_table", diff)

	// SQLite returns warning steps for column modifications
	if len(steps) == 0 {
		t.Error("Expected at least one warning step")
	}
}

func TestDriver_AddIndex(t *testing.T) {
	driver := NewDriver()

	idx := database.Index{
		Name:    "idx_test",
		Columns: []string{"col1"},
		Unique:  false,
	}

	sql, desc := driver.AddIndex("test_table", idx)

	if sql == "" {
		t.Error("Expected non-empty SQL")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_DropIndex(t *testing.T) {
	driver := NewDriver()

	idx := database.Index{Name: "idx_test"}
	sql, desc := driver.DropIndex("test_table", idx)

	if sql == "" {
		t.Error("Expected non-empty SQL")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_AddForeignKey(t *testing.T) {
	driver := NewDriver()

	fk := database.ForeignKey{
		Name:              "fk_test",
		Columns:           []string{"col1"},
		ReferencedTable:   "other_table",
		ReferencedColumns: []string{"id"},
	}

	sql, desc := driver.AddForeignKey("test_table", fk)

	// SQLite returns warning comment for foreign key operations
	if sql == "" {
		t.Error("Expected non-empty SQL (even if it's a comment)")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_DropForeignKey(t *testing.T) {
	driver := NewDriver()

	fk := database.ForeignKey{Name: "fk_test"}
	sql, desc := driver.DropForeignKey("test_table", fk)

	// SQLite returns warning comment for foreign key operations
	if sql == "" {
		t.Error("Expected non-empty SQL (even if it's a comment)")
	}

	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestDriver_FormatColumnDefinition(t *testing.T) {
	driver := NewDriver()

	col := database.Column{
		Name:     "test_col",
		Type:     "text",
		Nullable: false,
	}

	result := driver.FormatColumnDefinition(col)

	if result == "" {
		t.Error("Expected non-empty column definition")
	}
}

func TestDriver_ParameterPlaceholder(t *testing.T) {
	driver := NewDriver()

	result := driver.ParameterPlaceholder(1)

	if result != "?" {
		t.Errorf("Expected '?', got '%s'", result)
	}

	// SQLite uses ? for all positions
	result2 := driver.ParameterPlaceholder(5)
	if result2 != "?" {
		t.Errorf("Expected '?', got '%s'", result2)
	}
}
