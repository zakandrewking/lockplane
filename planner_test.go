package main

import (
	"strings"
	"testing"
)

func TestGeneratePlan_AddTable(t *testing.T) {
	diff := &SchemaDiff{
		AddedTables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "email", Type: "text", Nullable: false},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if !strings.Contains(step.SQL, "CREATE TABLE users") {
		t.Errorf("Expected CREATE TABLE in SQL, got: %s", step.SQL)
	}

	if !strings.Contains(step.SQL, "id integer NOT NULL PRIMARY KEY") {
		t.Errorf("Expected id column definition in SQL, got: %s", step.SQL)
	}

	if !strings.Contains(step.SQL, "email text NOT NULL") {
		t.Errorf("Expected email column definition in SQL, got: %s", step.SQL)
	}
}

func TestGeneratePlan_DropTable(t *testing.T) {
	diff := &SchemaDiff{
		RemovedTables: []Table{
			{Name: "old_table"},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.SQL != "DROP TABLE old_table CASCADE" {
		t.Errorf("Expected 'DROP TABLE old_table CASCADE', got: %s", step.SQL)
	}
}

func TestGeneratePlan_AddColumn(t *testing.T) {
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				AddedColumns: []Column{
					{Name: "age", Type: "integer", Nullable: true},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if !strings.Contains(step.SQL, "ALTER TABLE users ADD COLUMN age integer") {
		t.Errorf("Expected ALTER TABLE ADD COLUMN, got: %s", step.SQL)
	}

	if strings.Contains(step.SQL, "NOT NULL") {
		t.Errorf("Expected nullable column (no NOT NULL), got: %s", step.SQL)
	}
}

func TestGeneratePlan_DropColumn(t *testing.T) {
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				RemovedColumns: []Column{
					{Name: "deprecated_field"},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.SQL != "ALTER TABLE users DROP COLUMN deprecated_field" {
		t.Errorf("Expected 'ALTER TABLE users DROP COLUMN deprecated_field', got: %s", step.SQL)
	}
}

func TestGeneratePlan_ModifyColumn_Type(t *testing.T) {
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				ModifiedColumns: []ColumnDiff{
					{
						ColumnName: "age",
						Old:        Column{Name: "age", Type: "integer", Nullable: true},
						New:        Column{Name: "age", Type: "bigint", Nullable: true},
						Changes:    []string{"type"},
					},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.SQL != "ALTER TABLE users ALTER COLUMN age TYPE bigint" {
		t.Errorf("Expected type change SQL, got: %s", step.SQL)
	}
}

func TestGeneratePlan_ModifyColumn_Nullable(t *testing.T) {
	// Test setting NOT NULL
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				ModifiedColumns: []ColumnDiff{
					{
						ColumnName: "email",
						Old:        Column{Name: "email", Type: "text", Nullable: true},
						New:        Column{Name: "email", Type: "text", Nullable: false},
						Changes:    []string{"nullable"},
					},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.SQL != "ALTER TABLE users ALTER COLUMN email SET NOT NULL" {
		t.Errorf("Expected SET NOT NULL, got: %s", step.SQL)
	}

	// Test removing NOT NULL
	diff.ModifiedTables[0].ModifiedColumns[0].Old.Nullable = false
	diff.ModifiedTables[0].ModifiedColumns[0].New.Nullable = true

	plan, err = GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	step = plan.Steps[0]
	if step.SQL != "ALTER TABLE users ALTER COLUMN email DROP NOT NULL" {
		t.Errorf("Expected DROP NOT NULL, got: %s", step.SQL)
	}
}

func TestGeneratePlan_ModifyColumn_Default(t *testing.T) {
	defaultVal := "now()"

	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				ModifiedColumns: []ColumnDiff{
					{
						ColumnName: "created_at",
						Old:        Column{Name: "created_at", Type: "timestamp", Nullable: true},
						New:        Column{Name: "created_at", Type: "timestamp", Nullable: true, Default: &defaultVal},
						Changes:    []string{"default"},
					},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.SQL != "ALTER TABLE users ALTER COLUMN created_at SET DEFAULT now()" {
		t.Errorf("Expected SET DEFAULT, got: %s", step.SQL)
	}
}

func TestGeneratePlan_AddIndex(t *testing.T) {
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				AddedIndexes: []Index{
					{Name: "idx_users_email", Columns: []string{"email"}, Unique: true},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.SQL != "CREATE UNIQUE INDEX idx_users_email ON users (email)" {
		t.Errorf("Expected CREATE UNIQUE INDEX, got: %s", step.SQL)
	}
}

func TestGeneratePlan_DropIndex(t *testing.T) {
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				RemovedIndexes: []Index{
					{Name: "idx_old"},
				},
			},
		},
	}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.SQL != "DROP INDEX idx_old" {
		t.Errorf("Expected DROP INDEX, got: %s", step.SQL)
	}
}

func TestGeneratePlan_ComplexMigration(t *testing.T) {
	// Test a complex migration with multiple operations
	diff := &SchemaDiff{
		AddedTables: []Table{
			{
				Name: "posts",
				Columns: []Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "title", Type: "text", Nullable: false},
				},
			},
		},
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				AddedColumns: []Column{
					{Name: "age", Type: "integer", Nullable: true},
				},
				RemovedColumns: []Column{
					{Name: "old_field"},
				},
				AddedIndexes: []Index{
					{Name: "idx_users_age", Columns: []string{"age"}, Unique: false},
				},
			},
		},
		RemovedTables: []Table{
			{Name: "deprecated_table"},
		},
	}

	plan, err := GeneratePlan(diff)
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
	if !strings.Contains(plan.Steps[0].SQL, "CREATE TABLE posts") {
		t.Errorf("Step 0 should create table, got: %s", plan.Steps[0].SQL)
	}

	if !strings.Contains(plan.Steps[1].SQL, "ADD COLUMN age") {
		t.Errorf("Step 1 should add column, got: %s", plan.Steps[1].SQL)
	}

	if !strings.Contains(plan.Steps[2].SQL, "CREATE INDEX") {
		t.Errorf("Step 2 should create index, got: %s", plan.Steps[2].SQL)
	}

	if !strings.Contains(plan.Steps[3].SQL, "DROP COLUMN") {
		t.Errorf("Step 3 should drop column, got: %s", plan.Steps[3].SQL)
	}

	if !strings.Contains(plan.Steps[4].SQL, "DROP TABLE") {
		t.Errorf("Step 4 should drop table, got: %s", plan.Steps[4].SQL)
	}
}

func TestGeneratePlan_EmptyDiff(t *testing.T) {
	diff := &SchemaDiff{}

	plan, err := GeneratePlan(diff)
	if err != nil {
		t.Fatalf("Failed to generate plan: %v", err)
	}

	if len(plan.Steps) != 0 {
		t.Errorf("Expected empty plan for empty diff, got %d steps", len(plan.Steps))
	}
}
