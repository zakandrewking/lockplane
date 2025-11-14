package planner

import (
	"strings"
	"testing"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/database/postgres"
)

func TestGenerateRollback_CreateTable(t *testing.T) {
	beforeSchema := &database.Schema{Tables: []database.Table{}}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Create table users",
				SQL:         []string{"CREATE TABLE users (\n  id integer NOT NULL PRIMARY KEY,\n  email text NOT NULL\n)"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "DROP TABLE users") {
		t.Errorf("Expected DROP TABLE users, got: %v", step.SQL)
	}
}

func TestGenerateRollback_DropTable(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "old_table",
				Columns: []database.Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "name", Type: "text", Nullable: false},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Drop table old_table",
				SQL:         []string{"DROP TABLE old_table CASCADE"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "CREATE TABLE old_table") {
		t.Errorf("Expected CREATE TABLE old_table, got: %v", step.SQL)
	}
}

func TestGenerateRollback_AddColumn(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Add column email to table users",
				SQL:         []string{"ALTER TABLE users ADD COLUMN email text NOT NULL"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users DROP COLUMN email" {
		t.Errorf("Expected 'ALTER TABLE users DROP COLUMN email', got: %v", step.SQL)
	}
}

func TestGenerateRollback_DropColumn(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "deprecated_field", Type: "text", Nullable: true},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Drop column deprecated_field from table users",
				SQL:         []string{"ALTER TABLE users DROP COLUMN deprecated_field"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "ALTER TABLE users ADD COLUMN deprecated_field") {
		t.Errorf("Expected ADD COLUMN deprecated_field, got: %v", step.SQL)
	}
}

func TestGenerateRollback_AlterColumnType(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "age", Type: "integer", Nullable: true},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Change type of users.age from integer to bigint",
				SQL:         []string{"ALTER TABLE users ALTER COLUMN age TYPE bigint"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users ALTER COLUMN age TYPE integer" {
		t.Errorf("Expected TYPE integer, got: %v", step.SQL)
	}
}

func TestGenerateRollback_SetNotNull(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "email", Type: "text", Nullable: true},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Change nullability of users.email to false",
				SQL:         []string{"ALTER TABLE users ALTER COLUMN email SET NOT NULL"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "ALTER TABLE users ALTER COLUMN email DROP NOT NULL" {
		t.Errorf("Expected DROP NOT NULL, got: %v", step.SQL)
	}
}

func TestGenerateRollback_CreateIndex(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "email", Type: "text", Nullable: false},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Create index idx_users_email on table users",
				SQL:         []string{"CREATE UNIQUE INDEX idx_users_email ON users (email)"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || step.SQL[0] != "DROP INDEX idx_users_email" {
		t.Errorf("Expected DROP INDEX idx_users_email, got: %v", step.SQL)
	}
}

func TestGenerateRollback_DropIndex(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "email", Type: "text", Nullable: false},
				},
				Indexes: []database.Index{
					{Name: "idx_old", Columns: []string{"email"}, Unique: false},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Drop index idx_old from table users",
				SQL:         []string{"DROP INDEX idx_old"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "CREATE INDEX idx_old") {
		t.Errorf("Expected CREATE INDEX idx_old, got: %v", step.SQL)
	}
}

func TestGenerateRollback_ComplexMigration(t *testing.T) {
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
					{Name: "email", Type: "text", Nullable: false},
				},
			},
		},
	}

	// Forward: add table posts, add column age to users
	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Create table posts",
				SQL:         []string{"CREATE TABLE posts (\n  id integer NOT NULL PRIMARY KEY,\n  title text NOT NULL\n)"},
			},
			{
				Description: "Add column age to table users",
				SQL:         []string{"ALTER TABLE users ADD COLUMN age integer"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	// Rollback should have 2 steps in reverse order
	if len(rollbackPlan.Steps) != 2 {
		t.Fatalf("Expected 2 rollback steps, got %d", len(rollbackPlan.Steps))
	}

	// First rollback step should remove the age column (reverse of last forward step)
	if len(rollbackPlan.Steps[0].SQL) == 0 || !strings.Contains(rollbackPlan.Steps[0].SQL[0], "DROP COLUMN age") {
		t.Errorf("First rollback step should drop age column, got: %v", rollbackPlan.Steps[0].SQL)
	}

	// Second rollback step should drop the posts table (reverse of first forward step)
	if len(rollbackPlan.Steps[1].SQL) == 0 || !strings.Contains(rollbackPlan.Steps[1].SQL[0], "DROP TABLE posts") {
		t.Errorf("Second rollback step should drop posts table, got: %v", rollbackPlan.Steps[1].SQL)
	}
}

func TestGenerateRollback_SetDefault(t *testing.T) {
	defaultVal := "0"
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "score", Type: "integer", Nullable: true, Default: &defaultVal},
				},
			},
		},
	}

	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Change default of users.score",
				SQL:         []string{"ALTER TABLE users ALTER COLUMN score SET DEFAULT 100"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "SET DEFAULT 0") {
		t.Errorf("Expected SET DEFAULT 0, got: %v", step.SQL)
	}
}

func TestGenerateRollback_AddForeignKey(t *testing.T) {
	// Before schema: table without foreign key
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "posts",
				Columns: []database.Column{
					{Name: "id", Type: "bigint", IsPrimaryKey: true},
					{Name: "user_id", Type: "bigint"},
				},
				ForeignKeys: []database.ForeignKey{}, // No foreign keys initially
			},
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "bigint", IsPrimaryKey: true},
				},
			},
		},
	}

	// Forward plan: ADD foreign key
	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Add foreign key fk_posts_user to table posts",
				SQL:         []string{"ALTER TABLE posts ADD CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users (id)"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "DROP CONSTRAINT fk_posts_user") {
		t.Errorf("Expected DROP CONSTRAINT fk_posts_user, got: %v", step.SQL)
	}
}

func TestGenerateRollback_DropForeignKey(t *testing.T) {
	onDelete := "CASCADE"

	// Before schema: table with foreign key
	beforeSchema := &database.Schema{
		Tables: []database.Table{
			{
				Name: "posts",
				Columns: []database.Column{
					{Name: "id", Type: "bigint", IsPrimaryKey: true},
					{Name: "user_id", Type: "bigint"},
				},
				ForeignKeys: []database.ForeignKey{
					{
						Name:              "fk_posts_user",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
						OnDelete:          &onDelete,
					},
				},
			},
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "bigint", IsPrimaryKey: true},
				},
			},
		},
	}

	// Forward plan: DROP foreign key
	forwardPlan := &Plan{
		Steps: []PlanStep{
			{
				Description: "Drop foreign key fk_posts_user from table posts",
				SQL:         []string{"ALTER TABLE posts DROP CONSTRAINT fk_posts_user"},
			},
		},
	}

	driver := postgres.NewDriver()
	rollbackPlan, err := GenerateRollback(forwardPlan, beforeSchema, driver)
	if err != nil {
		t.Fatalf("Failed to generate rollback: %v", err)
	}

	if len(rollbackPlan.Steps) != 1 {
		t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
	}

	step := rollbackPlan.Steps[0]
	if len(step.SQL) == 0 || !strings.Contains(step.SQL[0], "ADD CONSTRAINT fk_posts_user") {
		t.Errorf("Expected ADD CONSTRAINT fk_posts_user, got: %v", step.SQL)
	}

	// Verify ON DELETE CASCADE is preserved
	if !strings.Contains(step.SQL[0], "ON DELETE CASCADE") {
		t.Errorf("Expected ON DELETE CASCADE in foreign key, got: %v", step.SQL)
	}
}
