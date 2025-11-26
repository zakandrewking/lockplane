package locks_test

import (
	"github.com/lockplane/lockplane/internal/locks"
	"strings"
	"testing"

	"github.com/lockplane/lockplane/internal/planner"
)

func TestGenerateSaferRewrite_CreateIndex(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		shouldRewrite bool
		expectedSQL   string
	}{
		{
			name:          "CREATE INDEX should rewrite",
			sql:           "CREATE INDEX idx_users_email ON users(email)",
			shouldRewrite: true,
			expectedSQL:   "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
		},
		{
			name:          "CREATE UNIQUE INDEX should rewrite",
			sql:           "CREATE UNIQUE INDEX idx_users_email ON users(email)",
			shouldRewrite: true,
			expectedSQL:   "CREATE UNIQUE INDEX CONCURRENTLY idx_users_email ON users(email)",
		},
		{
			name:          "CREATE INDEX CONCURRENTLY should not rewrite",
			sql:           "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
			shouldRewrite: false,
		},
		{
			name:          "CREATE UNIQUE INDEX CONCURRENTLY should not rewrite",
			sql:           "CREATE UNIQUE INDEX CONCURRENTLY idx_users_email ON users(email)",
			shouldRewrite: false,
		},
		{
			name:          "Lowercase should rewrite",
			sql:           "create index idx_email on users(email)",
			shouldRewrite: true,
			expectedSQL:   "create index CONCURRENTLY idx_email on users(email)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{
				Description: tt.name,
				SQL:         []string{tt.sql},
			}

			rewrite := planner.GenerateSaferRewrite(step)

			if tt.shouldRewrite {
				if rewrite == nil {
					t.Fatal("Expected rewrite but got nil")
				}

				if len(rewrite.SQL) != 1 {
					t.Errorf("Expected 1 SQL statement, got %d", len(rewrite.SQL))
				}

				if rewrite.SQL[0] != tt.expectedSQL {
					t.Errorf("SQL = %q, want %q", rewrite.SQL[0], tt.expectedSQL)
				}

				if rewrite.LockMode != locks.LockShareUpdateExclusive {
					t.Errorf("locks.LockMode = %v, want %v", rewrite.LockMode, locks.LockShareUpdateExclusive)
				}

				if !strings.Contains(rewrite.Description, "CONCURRENTLY") {
					t.Errorf("Description should mention CONCURRENTLY")
				}

				if len(rewrite.Tradeoffs) == 0 {
					t.Error("Should have tradeoffs")
				}
			} else {
				if rewrite != nil {
					t.Errorf("Expected no rewrite but got: %+v", rewrite)
				}
			}
		})
	}
}

func TestGenerateSaferRewrite_AddConstraint(t *testing.T) {
	tests := []struct {
		name                   string
		sql                    string
		shouldRewrite          bool
		expectedPhase1Contains string
		expectedPhase2Contains string
	}{
		{
			name:                   "ADD CONSTRAINT CHECK should rewrite",
			sql:                    "ALTER TABLE orders ADD CONSTRAINT check_positive CHECK (amount > 0)",
			shouldRewrite:          true,
			expectedPhase1Contains: "NOT VALID",
			expectedPhase2Contains: "VALIDATE CONSTRAINT",
		},
		{
			name:                   "ADD CONSTRAINT UNIQUE should rewrite",
			sql:                    "ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email)",
			shouldRewrite:          true,
			expectedPhase1Contains: "NOT VALID",
			expectedPhase2Contains: "VALIDATE CONSTRAINT users_email_unique",
		},
		{
			name:          "ADD CONSTRAINT NOT VALID should not rewrite",
			sql:           "ALTER TABLE orders ADD CONSTRAINT check_positive CHECK (amount > 0) NOT VALID",
			shouldRewrite: false,
		},
		{
			name:          "VALIDATE CONSTRAINT should not rewrite",
			sql:           "ALTER TABLE orders VALIDATE CONSTRAINT check_positive",
			shouldRewrite: false,
		},
		{
			name:                   "Lowercase should rewrite",
			sql:                    "alter table users add constraint chk check (x > 0)",
			shouldRewrite:          true,
			expectedPhase1Contains: "NOT VALID",
			expectedPhase2Contains: "VALIDATE CONSTRAINT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{
				Description: tt.name,
				SQL:         []string{tt.sql},
			}

			rewrite := planner.GenerateSaferRewrite(step)

			if tt.shouldRewrite {
				if rewrite == nil {
					t.Fatal("Expected rewrite but got nil")
				}

				if len(rewrite.SQL) != 2 {
					t.Fatalf("Expected 2 SQL statements, got %d", len(rewrite.SQL))
				}

				// Check phase 1 SQL
				if !strings.Contains(rewrite.SQL[0], tt.expectedPhase1Contains) {
					t.Errorf("Phase 1 SQL should contain %q, got: %s", tt.expectedPhase1Contains, rewrite.SQL[0])
				}

				// Check phase 2 SQL
				if !strings.Contains(rewrite.SQL[1], tt.expectedPhase2Contains) {
					t.Errorf("Phase 2 SQL should contain %q, got: %s", tt.expectedPhase2Contains, rewrite.SQL[1])
				}

				if rewrite.LockMode != locks.LockShareUpdateExclusive {
					t.Errorf("locks.LockMode = %v, want %v", rewrite.LockMode, locks.LockShareUpdateExclusive)
				}

				if !rewrite.RequiresMultipleSteps {
					t.Error("Should require multiple steps")
				}

				if len(rewrite.Tradeoffs) == 0 {
					t.Error("Should have tradeoffs")
				}
			} else {
				if rewrite != nil {
					t.Errorf("Expected no rewrite but got: %+v", rewrite)
				}
			}
		})
	}
}

func TestGenerateSaferRewrite_AlterColumnType(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		shouldSuggest bool
	}{
		{
			name:          "ALTER COLUMN TYPE should suggest multi-phase",
			sql:           "ALTER TABLE users ALTER COLUMN age TYPE INTEGER",
			shouldSuggest: true,
		},
		{
			name:          "ALTER COLUMN TYPE with USING should suggest multi-phase",
			sql:           "ALTER TABLE users ALTER COLUMN age TYPE INTEGER USING age::INTEGER",
			shouldSuggest: true,
		},
		{
			name:          "ALTER COLUMN without TYPE should not suggest",
			sql:           "ALTER TABLE users ALTER COLUMN email SET NOT NULL",
			shouldSuggest: false,
		},
		{
			name:          "Lowercase should suggest",
			sql:           "alter table users alter column age type bigint",
			shouldSuggest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{
				Description: tt.name,
				SQL:         []string{tt.sql},
			}

			rewrite := planner.GenerateSaferRewrite(step)

			if tt.shouldSuggest {
				if rewrite == nil {
					t.Fatal("Expected multi-phase suggestion but got nil")
				}

				if rewrite.SQL != nil {
					t.Error("Multi-phase suggestion should have nil SQL")
				}

				if !strings.Contains(rewrite.Description, "multi-phase") {
					t.Error("Description should mention multi-phase")
				}

				if !rewrite.RequiresMultipleSteps {
					t.Error("Should require multiple steps")
				}

				if !strings.Contains(rewrite.Notes, "plan-multiphase") {
					t.Error("Notes should mention plan-multiphase command")
				}

				if len(rewrite.Tradeoffs) == 0 {
					t.Error("Should have tradeoffs")
				}
			} else {
				if rewrite != nil {
					t.Errorf("Expected no suggestion but got: %+v", rewrite)
				}
			}
		})
	}
}

func TestGenerateSaferRewrite_NoRewrite(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{"SELECT statement", "SELECT * FROM users"},
		{"INSERT statement", "INSERT INTO users (email) VALUES ('test@example.com')"},
		{"CREATE TABLE", "CREATE TABLE users (id BIGINT PRIMARY KEY)"},
		{"DROP TABLE", "DROP TABLE users"},
		{"ALTER TABLE ADD COLUMN", "ALTER TABLE users ADD COLUMN email TEXT"},
		{"Empty SQL", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{
				Description: tt.name,
				SQL:         []string{tt.sql},
			}

			rewrite := planner.GenerateSaferRewrite(step)
			if rewrite != nil {
				t.Errorf("Expected no rewrite for %q but got: %+v", tt.name, rewrite)
			}
		})
	}
}

func TestInjectLockTimeout(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		timeoutSeconds   int
		expectedContains []string
	}{
		{
			name:           "Add lock timeout",
			sql:            "ALTER TABLE users ADD COLUMN email TEXT",
			timeoutSeconds: 2,
			expectedContains: []string{
				"SET lock_timeout = '2s'",
				"ALTER TABLE users ADD COLUMN email TEXT",
			},
		},
		{
			name:           "Handle semicolon",
			sql:            "ALTER TABLE users ADD COLUMN email TEXT;",
			timeoutSeconds: 5,
			expectedContains: []string{
				"SET lock_timeout = '5s'",
				"ALTER TABLE users ADD COLUMN email TEXT",
			},
		},
		{
			name:             "Zero timeout should return original",
			sql:              "ALTER TABLE users ADD COLUMN email TEXT",
			timeoutSeconds:   0,
			expectedContains: []string{"ALTER TABLE users ADD COLUMN email TEXT"},
		},
		{
			name:             "Negative timeout should return original",
			sql:              "ALTER TABLE users ADD COLUMN email TEXT",
			timeoutSeconds:   -1,
			expectedContains: []string{"ALTER TABLE users ADD COLUMN email TEXT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := locks.InjectLockTimeout(tt.sql, tt.timeoutSeconds)

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("Result should contain %q, got: %s", expected, result)
				}
			}

			if tt.timeoutSeconds <= 0 {
				if result != tt.sql {
					t.Errorf("Result should equal input for invalid timeout, got: %s", result)
				}
			}
		})
	}
}

func TestExtractTableName(t *testing.T) {
	tests := []struct {
		sql      string
		expected string
	}{
		{"ALTER TABLE users ADD COLUMN email TEXT", "users"},
		{"ALTER TABLE orders DROP COLUMN status", "orders"},
		{"alter table products add constraint chk check (x > 0)", "products"},
		{"ALTER  TABLE  my_table  ADD COLUMN x INT", "my_table"},
		{"CREATE TABLE users (id INT)", ""}, // Not ALTER TABLE
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := locks.ExtractTableName(tt.sql)
			if got != tt.expected {
				t.Errorf("locks.ExtractTableName(%q) = %q, want %q", tt.sql, got, tt.expected)
			}
		})
	}
}

func TestExtractConstraintName(t *testing.T) {
	tests := []struct {
		sql      string
		expected string
	}{
		{
			"ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email)",
			"users_email_unique",
		},
		{
			"ALTER TABLE orders ADD CONSTRAINT check_positive CHECK (amount > 0)",
			"check_positive",
		},
		{
			"alter table products add constraint fk_category foreign key (category_id) references categories(id)",
			"fk_category",
		},
		{
			"ALTER TABLE users ADD CONSTRAINT CHECK (x > 0)", // No name
			"users_check", // Generated default
		},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := locks.ExtractConstraintName(tt.sql)
			if got != tt.expected {
				t.Errorf("locks.ExtractConstraintName(%q) = %q, want %q", tt.sql, got, tt.expected)
			}
		})
	}
}

func TestExtractColumnNameFromAlter(t *testing.T) {
	tests := []struct {
		sql      string
		expected string
	}{
		{"ALTER TABLE users ALTER COLUMN age TYPE INTEGER", "age"},
		{"ALTER TABLE orders ALTER COLUMN status SET NOT NULL", "status"},
		{"alter table products alter column price type numeric", "price"},
		{"ALTER  TABLE  users  ALTER  COLUMN  email_address  TYPE TEXT", "email_address"},
		{"ALTER TABLE users ADD COLUMN email TEXT", ""}, // Not ALTER COLUMN
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := locks.ExtractColumnNameFromAlter(tt.sql)
			if got != tt.expected {
				t.Errorf("locks.ExtractColumnNameFromAlter(%q) = %q, want %q", tt.sql, got, tt.expected)
			}
		})
	}
}

func TestShouldRewrite(t *testing.T) {
	tests := []struct {
		name     string
		impact   *locks.LockImpact
		expected bool
	}{
		{
			name: "High impact should rewrite",
			impact: &locks.LockImpact{
				Impact:              locks.ImpactHigh,
				EstimatedDurationMS: 100,
				BlocksWrites:        true,
			},
			expected: true,
		},
		{
			name: "Long duration should rewrite",
			impact: &locks.LockImpact{
				Impact:              locks.ImpactLow,
				EstimatedDurationMS: 2000,
				BlocksWrites:        false,
			},
			expected: true,
		},
		{
			name: "Blocks writes should rewrite",
			impact: &locks.LockImpact{
				Impact:              locks.ImpactMedium,
				EstimatedDurationMS: 500,
				BlocksWrites:        true,
			},
			expected: true,
		},
		{
			name: "Low impact and fast should not rewrite",
			impact: &locks.LockImpact{
				Impact:              locks.ImpactLow,
				EstimatedDurationMS: 100,
				BlocksWrites:        false,
			},
			expected: false,
		},
		{
			name: "None impact and fast should not rewrite",
			impact: &locks.LockImpact{
				Impact:              locks.ImpactNone,
				EstimatedDurationMS: 100,
				BlocksWrites:        false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := locks.ShouldRewrite(tt.impact)
			if got != tt.expected {
				t.Errorf("locks.ShouldRewrite() = %v, want %v", got, tt.expected)
			}
		})
	}
}
