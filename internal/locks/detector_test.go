package locks

import (
	"testing"

	"github.com/lockplane/lockplane/internal/planner"
)

func TestDetectLockMode(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		expectedLock LockMode
	}{
		// CREATE INDEX patterns
		{
			name:         "CREATE INDEX (non-concurrent)",
			sql:          "CREATE INDEX idx_users_email ON users(email)",
			expectedLock: LockShare,
		},
		{
			name:         "CREATE UNIQUE INDEX (non-concurrent)",
			sql:          "CREATE UNIQUE INDEX idx_users_email ON users(email)",
			expectedLock: LockShare,
		},
		{
			name:         "CREATE INDEX CONCURRENTLY",
			sql:          "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
			expectedLock: LockShareUpdateExclusive,
		},
		{
			name:         "CREATE UNIQUE INDEX CONCURRENTLY",
			sql:          "CREATE UNIQUE INDEX CONCURRENTLY idx_users_email ON users(email)",
			expectedLock: LockShareUpdateExclusive,
		},

		// ALTER TABLE patterns
		{
			name:         "ALTER TABLE ADD COLUMN",
			sql:          "ALTER TABLE users ADD COLUMN email TEXT",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "ALTER TABLE DROP COLUMN",
			sql:          "ALTER TABLE users DROP COLUMN email",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "ALTER TABLE ALTER COLUMN TYPE",
			sql:          "ALTER TABLE users ALTER COLUMN age TYPE BIGINT",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "ALTER TABLE ADD CONSTRAINT",
			sql:          "ALTER TABLE users ADD CONSTRAINT check_positive CHECK (amount > 0)",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "ALTER TABLE ADD CONSTRAINT NOT VALID",
			sql:          "ALTER TABLE users ADD CONSTRAINT check_positive CHECK (amount > 0) NOT VALID",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "ALTER TABLE VALIDATE CONSTRAINT",
			sql:          "ALTER TABLE users VALIDATE CONSTRAINT check_positive",
			expectedLock: LockShareUpdateExclusive,
		},

		// DROP patterns
		{
			name:         "DROP TABLE",
			sql:          "DROP TABLE users",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "DROP INDEX",
			sql:          "DROP INDEX idx_users_email",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "TRUNCATE",
			sql:          "TRUNCATE TABLE users",
			expectedLock: LockAccessExclusive,
		},

		// CREATE TABLE
		{
			name:         "CREATE TABLE",
			sql:          "CREATE TABLE users (id BIGINT PRIMARY KEY)",
			expectedLock: LockAccessShare,
		},

		// DML patterns
		{
			name:         "INSERT",
			sql:          "INSERT INTO users (email) VALUES ('test@example.com')",
			expectedLock: LockRowExclusive,
		},
		{
			name:         "UPDATE",
			sql:          "UPDATE users SET email = 'new@example.com' WHERE id = 1",
			expectedLock: LockRowExclusive,
		},
		{
			name:         "DELETE",
			sql:          "DELETE FROM users WHERE id = 1",
			expectedLock: LockRowExclusive,
		},

		// SELECT
		{
			name:         "SELECT",
			sql:          "SELECT * FROM users",
			expectedLock: LockAccessShare,
		},

		// Edge cases
		{
			name:         "Empty SQL",
			sql:          "",
			expectedLock: LockAccessShare,
		},
		{
			name:         "Lowercase SQL",
			sql:          "alter table users add column email text",
			expectedLock: LockAccessExclusive,
		},
		{
			name:         "Mixed case SQL",
			sql:          "Create Index Concurrently idx_email ON users(email)",
			expectedLock: LockShareUpdateExclusive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{
				Description: tt.name,
				SQL:         []string{tt.sql},
			}

			got := DetectLockMode(step)
			if got != tt.expectedLock {
				t.Errorf("DetectLockMode() = %v (%s), want %v (%s)",
					got, got.String(), tt.expectedLock, tt.expectedLock.String())
			}
		})
	}
}

func TestDetectLockMode_NoSQL(t *testing.T) {
	step := planner.PlanStep{
		Description: "No SQL step",
		SQL:         []string{},
	}

	got := DetectLockMode(step)
	if got != LockAccessShare {
		t.Errorf("DetectLockMode() for empty SQL = %v, want %v", got, LockAccessShare)
	}
}

func TestAnalyzeLockImpact(t *testing.T) {
	tests := []struct {
		name                 string
		sql                  string
		expectedLockMode     LockMode
		expectedBlocksReads  bool
		expectedBlocksWrites bool
	}{
		{
			name:                 "CREATE INDEX blocks writes",
			sql:                  "CREATE INDEX idx_users_email ON users(email)",
			expectedLockMode:     LockShare,
			expectedBlocksReads:  false,
			expectedBlocksWrites: true,
		},
		{
			name:                 "CREATE INDEX CONCURRENTLY allows all",
			sql:                  "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
			expectedLockMode:     LockShareUpdateExclusive,
			expectedBlocksReads:  false,
			expectedBlocksWrites: false,
		},
		{
			name:                 "ALTER TABLE blocks everything",
			sql:                  "ALTER TABLE users ADD COLUMN email TEXT",
			expectedLockMode:     LockAccessExclusive,
			expectedBlocksReads:  true,
			expectedBlocksWrites: true,
		},
		{
			name:                 "SELECT blocks nothing",
			sql:                  "SELECT * FROM users",
			expectedLockMode:     LockAccessShare,
			expectedBlocksReads:  false,
			expectedBlocksWrites: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{
				Description: tt.name,
				SQL:         []string{tt.sql},
			}

			impact := AnalyzeLockImpact(step)

			if impact.LockMode != tt.expectedLockMode {
				t.Errorf("LockMode = %v, want %v", impact.LockMode, tt.expectedLockMode)
			}

			if impact.BlocksReads != tt.expectedBlocksReads {
				t.Errorf("BlocksReads = %v, want %v", impact.BlocksReads, tt.expectedBlocksReads)
			}

			if impact.BlocksWrites != tt.expectedBlocksWrites {
				t.Errorf("BlocksWrites = %v, want %v", impact.BlocksWrites, tt.expectedBlocksWrites)
			}

			if impact.Operation != tt.name {
				t.Errorf("Operation = %v, want %v", impact.Operation, tt.name)
			}

			if impact.Explanation == "" {
				t.Error("Explanation should not be empty")
			}
		})
	}
}

func TestIsCreateIndexConcurrently(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{"CREATE INDEX CONCURRENTLY", "CREATE INDEX CONCURRENTLY idx_email ON users(email)", true},
		{"CREATE UNIQUE INDEX CONCURRENTLY", "CREATE UNIQUE INDEX CONCURRENTLY idx_email ON users(email)", true},
		{"CREATE INDEX (non-concurrent)", "CREATE INDEX idx_email ON users(email)", false},
		{"ALTER TABLE", "ALTER TABLE users ADD COLUMN email TEXT", false},
		{"Empty SQL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{SQL: []string{tt.sql}}
			if got := IsCreateIndexConcurrently(step); got != tt.expected {
				t.Errorf("IsCreateIndexConcurrently() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsAddConstraintNotValid(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{"ADD CONSTRAINT NOT VALID", "ALTER TABLE users ADD CONSTRAINT chk CHECK (x > 0) NOT VALID", true},
		{"ADD CONSTRAINT (valid)", "ALTER TABLE users ADD CONSTRAINT chk CHECK (x > 0)", false},
		{"Other ALTER TABLE", "ALTER TABLE users ADD COLUMN email TEXT", false},
		{"Empty SQL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{SQL: []string{tt.sql}}
			if got := IsAddConstraintNotValid(step); got != tt.expected {
				t.Errorf("IsAddConstraintNotValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsValidateConstraint(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{"VALIDATE CONSTRAINT", "ALTER TABLE users VALIDATE CONSTRAINT chk", true},
		{"ADD CONSTRAINT", "ALTER TABLE users ADD CONSTRAINT chk CHECK (x > 0)", false},
		{"Other ALTER TABLE", "ALTER TABLE users ADD COLUMN email TEXT", false},
		{"Empty SQL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{SQL: []string{tt.sql}}
			if got := IsValidateConstraint(step); got != tt.expected {
				t.Errorf("IsValidateConstraint() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExplainLockMode(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		shouldContain string
	}{
		{
			name:          "CREATE INDEX explains SHARE lock",
			sql:           "CREATE INDEX idx_email ON users(email)",
			shouldContain: "blocking writes",
		},
		{
			name:          "CREATE INDEX CONCURRENTLY explains concurrent",
			sql:           "CREATE INDEX CONCURRENTLY idx_email ON users(email)",
			shouldContain: "concurrent reads and writes",
		},
		{
			name:          "ALTER TABLE ADD COLUMN explains exclusive",
			sql:           "ALTER TABLE users ADD COLUMN email TEXT",
			shouldContain: "exclusive access",
		},
		{
			name:          "DROP COLUMN explains exclusive",
			sql:           "ALTER TABLE users DROP COLUMN email",
			shouldContain: "exclusive access",
		},
		{
			name:          "VALIDATE CONSTRAINT explains concurrent",
			sql:           "ALTER TABLE users VALIDATE CONSTRAINT chk",
			shouldContain: "concurrent reads and writes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := planner.PlanStep{
				Description: tt.name,
				SQL:         []string{tt.sql},
			}

			lockMode := DetectLockMode(step)
			explanation := explainLockMode(step, lockMode)

			if !containsSubstring(explanation, tt.shouldContain) {
				t.Errorf("Explanation %q should contain %q", explanation, tt.shouldContain)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) && indexOfSubstring(s, substr) >= 0)
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
