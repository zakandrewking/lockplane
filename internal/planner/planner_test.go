package planner

import (
	"strings"
	"testing"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/database/postgres"
	"github.com/lockplane/lockplane/database/sqlite"
	"github.com/lockplane/lockplane/internal/schema"
)

func TestGeneratePlan_AddTable(t *testing.T) {
	diff := &schema.SchemaDiff{
		AddedTables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "email", Type: "text", Nullable: false},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "CREATE TABLE users") {
		t.Errorf("Expected CREATE TABLE in SQL, got: %v", step.SQL)
	}

	if !strings.Contains(step.SQL[0], "id integer NOT NULL PRIMARY KEY") {
		t.Errorf("Expected id column definition in SQL, got: %s", step.SQL[0])
	}

	if !strings.Contains(step.SQL[0], "email text NOT NULL") {
		t.Errorf("Expected email column definition in SQL, got: %s", step.SQL[0])
	}
}

func TestGeneratePlan_DropTable(t *testing.T) {
	diff := &schema.SchemaDiff{
		RemovedTables: []database.Table{
			{Name: "old_table"},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "DROP TABLE old_table CASCADE" {
		t.Errorf("Expected 'DROP TABLE old_table CASCADE', got: %v", step.SQL)
	}
}

func TestGeneratePlan_AddColumn(t *testing.T) {
	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				AddedColumns: []database.Column{
					{Name: "age", Type: "integer", Nullable: true},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "ALTER TABLE users ADD COLUMN age integer") {
		t.Errorf("Expected ALTER TABLE ADD COLUMN, got: %v", step.SQL)
	}

	if strings.Contains(step.SQL[0], "NOT NULL") {
		t.Errorf("Expected nullable column (no NOT NULL), got: %s", step.SQL[0])
	}
}

func TestGeneratePlan_DropColumn(t *testing.T) {
	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				RemovedColumns: []database.Column{
					{Name: "deprecated_field"},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users DROP COLUMN deprecated_field" {
		t.Errorf("Expected 'ALTER TABLE users DROP COLUMN deprecated_field', got: %v", step.SQL)
	}
}

func TestGeneratePlan_ModifyColumn_Type(t *testing.T) {
	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				ModifiedColumns: []schema.ColumnDiff{
					{
						ColumnName: "age",
						Old:        database.Column{Name: "age", Type: "integer", Nullable: true},
						New:        database.Column{Name: "age", Type: "bigint", Nullable: true},
						Changes:    []string{"type"},
					},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users ALTER COLUMN age TYPE bigint" {
		t.Errorf("Expected type change SQL, got: %v", step.SQL)
	}
}

func TestGeneratePlan_ModifyColumn_Nullable(t *testing.T) {
	// Test setting NOT NULL
	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				ModifiedColumns: []schema.ColumnDiff{
					{
						ColumnName: "email",
						Old:        database.Column{Name: "email", Type: "text", Nullable: true},
						New:        database.Column{Name: "email", Type: "text", Nullable: false},
						Changes:    []string{"nullable"},
					},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users ALTER COLUMN email SET NOT NULL" {
		t.Errorf("Expected SET NOT NULL, got: %v", step.SQL)
	}

	// Test removing NOT NULL
	diff.ModifiedTables[0].ModifiedColumns[0].Old.Nullable = false
	diff.ModifiedTables[0].ModifiedColumns[0].New.Nullable = true

	plan, err = GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	step = plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users ALTER COLUMN email DROP NOT NULL" {
		t.Errorf("Expected DROP NOT NULL, got: %v", step.SQL)
	}
}

func TestGeneratePlan_ModifyColumn_Default(t *testing.T) {
	defaultVal := "now()"

	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				ModifiedColumns: []schema.ColumnDiff{
					{
						ColumnName: "created_at",
						Old:        database.Column{Name: "created_at", Type: "timestamp", Nullable: true},
						New:        database.Column{Name: "created_at", Type: "timestamp", Nullable: true, Default: &defaultVal},
						Changes:    []string{"default"},
					},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users ALTER COLUMN created_at SET DEFAULT now()" {
		t.Errorf("Expected SET DEFAULT, got: %v", step.SQL)
	}
}

func TestGeneratePlan_AddIndex(t *testing.T) {
	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				AddedIndexes: []database.Index{
					{Name: "idx_users_email", Columns: []string{"email"}, Unique: true},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "CREATE UNIQUE INDEX idx_users_email ON users (email)" {
		t.Errorf("Expected CREATE UNIQUE INDEX, got: %v", step.SQL)
	}
}

func TestGeneratePlan_DropIndex(t *testing.T) {
	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				RemovedIndexes: []database.Index{
					{Name: "idx_old"},
				},
			},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "DROP INDEX idx_old" {
		t.Errorf("Expected DROP INDEX, got: %v", step.SQL)
	}
}

func TestGeneratePlan_ComplexMigration(t *testing.T) {
	// Test a complex migration with multiple operations
	diff := &schema.SchemaDiff{
		AddedTables: []database.Table{
			{
				Name: "posts",
				Columns: []database.Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "title", Type: "text", Nullable: false},
				},
			},
		},
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				AddedColumns: []database.Column{
					{Name: "age", Type: "integer", Nullable: true},
				},
				RemovedColumns: []database.Column{
					{Name: "old_field"},
				},
				AddedIndexes: []database.Index{
					{Name: "idx_users_age", Columns: []string{"age"}, Unique: false},
				},
			},
		},
		RemovedTables: []database.Table{
			{Name: "deprecated_table"},
		},
	}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	// Should have 5 steps:
	// 1. CREATE TABLE posts
	// 2. ALTER TABLE users ADD COLUMN age
	// 3. CREATE INDEX idx_users_age
	// 4. ALTER TABLE users DROP COLUMN old_field
	// 5. DROP TABLE deprecated_table
	if len(plan.Steps) != 5 {
		t.Fatalf("Expected 5 steps, got %d", len(plan.Steps))
	}

	// Verify order: adds before drops
	if len(plan.Steps[0].SQL) == 0 || !strings.Contains(plan.Steps[0].SQL[0], "CREATE TABLE posts") {
		t.Errorf("Step 0 should create table, got: %v", plan.Steps[0].SQL)
	}

	if len(plan.Steps[1].SQL) == 0 || !strings.Contains(plan.Steps[1].SQL[0], "ADD COLUMN age") {
		t.Errorf("Step 1 should add column, got: %v", plan.Steps[1].SQL)
	}

	if len(plan.Steps[2].SQL) == 0 || !strings.Contains(plan.Steps[2].SQL[0], "CREATE INDEX") {
		t.Errorf("Step 2 should create index, got: %v", plan.Steps[2].SQL)
	}

	if len(plan.Steps[3].SQL) == 0 || !strings.Contains(plan.Steps[3].SQL[0], "DROP COLUMN") {
		t.Errorf("Step 3 should drop column, got: %v", plan.Steps[3].SQL)
	}

	if len(plan.Steps[4].SQL) == 0 || !strings.Contains(plan.Steps[4].SQL[0], "DROP TABLE") {
		t.Errorf("Step 4 should drop table, got: %v", plan.Steps[4].SQL)
	}
}

func TestGeneratePlan_EmptyDiff(t *testing.T) {
	diff := &schema.SchemaDiff{}

	driver := postgres.NewDriver()
	plan, err := GeneratePlan(diff, driver)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 0 {
		t.Errorf("Expected empty plan for empty diff, got %d steps", len(plan.Steps))
	}
}

// TestGeneratePlan_PostgreSQLvsSQLite verifies that different drivers generate appropriate SQL
func TestGeneratePlan_PostgreSQLvsSQLite(t *testing.T) {
	diff := &schema.SchemaDiff{
		ModifiedTables: []schema.TableDiff{
			{
				TableName: "users",
				ModifiedColumns: []schema.ColumnDiff{
					{
						ColumnName: "age",
						Old:        database.Column{Name: "age", Type: "integer", Nullable: true},
						New:        database.Column{Name: "age", Type: "bigint", Nullable: true},
						Changes:    []string{"type"},
					},
				},
			},
		},
	}

	t.Run("PostgreSQL", func(t *testing.T) {
		driver := postgres.NewDriver()
		plan, err := GeneratePlan(diff, driver)
		if err != nil {
			t.Fatalf("Failed to generate PostgreSQL plan: %v", err)
		}

		if len(plan.Steps) == 0 {
			t.Fatal("Expected PostgreSQL to generate steps for column type change")
		}

		// PostgreSQL should use ALTER COLUMN TYPE
		foundAlterColumn := false
		for _, step := range plan.Steps {
			if len(step.SQL) > 0 && strings.Contains(step.SQL[0], "ALTER COLUMN") && strings.Contains(step.SQL[0], "TYPE") {
				foundAlterColumn = true
				break
			}
		}

		if !foundAlterColumn {
			t.Errorf("Expected PostgreSQL to use ALTER COLUMN TYPE, got steps: %v", plan.Steps)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		driver := sqlite.NewDriver()
		plan, err := GeneratePlan(diff, driver)
		if err != nil {
			t.Fatalf("Failed to generate SQLite plan: %v", err)
		}

		if len(plan.Steps) == 0 {
			t.Fatal("Expected SQLite to generate steps for column type change")
		}

		// Print what SQL was generated for debugging
		t.Logf("SQLite generated %d steps:", len(plan.Steps))
		for i, step := range plan.Steps {
			t.Logf("  Step %d: %s", i+1, step.Description)
			t.Logf("  SQL: %v", step.SQL)
		}

		// SQLite should NOT use ALTER COLUMN (it's not supported)
		// Instead it should use table recreation strategy or comment
		for _, step := range plan.Steps {
			for _, sqlStmt := range step.SQL {
				if strings.Contains(sqlStmt, "ALTER COLUMN") {
					t.Errorf("SQLite should not use ALTER COLUMN (not supported), got: %s", sqlStmt)
				}
			}
		}

		// SQLite should mention table recreation in description or use temp table
		foundRecreation := false
		for _, step := range plan.Steps {
			desc := strings.ToLower(step.Description)
			for _, sqlStmt := range step.SQL {
				sql := strings.ToLower(sqlStmt)
				if strings.Contains(desc, "recreat") || strings.Contains(desc, "rebuild") ||
					strings.Contains(sql, "_new") || strings.Contains(sql, "temp") {
					foundRecreation = true
					break
				}
			}
			if foundRecreation {
				break
			}
		}

		if !foundRecreation {
			t.Log("Note: SQLite migration strategy unclear from steps:")
			for i, step := range plan.Steps {
				t.Logf("  Step %d: %s", i+1, step.Description)
			}
		}
	})
}
