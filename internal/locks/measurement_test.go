package locks

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/lib/pq" // PostgreSQL driver for integration tests
	"github.com/lockplane/lockplane/internal/planner"
)

func TestMeasureLockDuration_NilDB(t *testing.T) {
	step := planner.PlanStep{
		Description: "Test step",
		SQL:         []string{"SELECT 1"},
	}

	_, err := MeasureLockDuration(context.Background(), nil, step)
	if err == nil {
		t.Error("Expected error for nil database")
	}
}

func TestMeasureLockDuration_NoSQL(t *testing.T) {
	// Use a mock or skip if no DB available
	step := planner.PlanStep{
		Description: "Test step",
		SQL:         []string{},
	}

	measurement, err := MeasureLockDuration(context.Background(), &sql.DB{}, step)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if measurement.Success {
		t.Error("Expected failure for empty SQL")
	}

	if measurement.Error == "" {
		t.Error("Expected error message")
	}
}

func TestMeasureLockDuration_EmptySQL(t *testing.T) {
	step := planner.PlanStep{
		Description: "Test step",
		SQL:         []string{""},
	}

	measurement, err := MeasureLockDuration(context.Background(), &sql.DB{}, step)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if measurement.Success {
		t.Error("Expected failure for empty SQL")
	}

	if !strings.Contains(measurement.Error, "empty SQL") {
		t.Errorf("Expected 'empty SQL' error, got: %s", measurement.Error)
	}
}

func TestExtractIndexName(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "CREATE INDEX",
			sql:      "CREATE INDEX idx_users_email ON users(email)",
			expected: "idx_users_email",
		},
		{
			name:     "CREATE UNIQUE INDEX",
			sql:      "CREATE UNIQUE INDEX idx_users_email ON users(email)",
			expected: "idx_users_email",
		},
		{
			name:     "CREATE INDEX CONCURRENTLY",
			sql:      "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
			expected: "idx_users_email",
		},
		{
			name:     "CREATE UNIQUE INDEX CONCURRENTLY",
			sql:      "CREATE UNIQUE INDEX CONCURRENTLY idx_users_email ON users(email)",
			expected: "idx_users_email",
		},
		{
			name:     "Lowercase",
			sql:      "create index idx_email on users(email)",
			expected: "idx_email",
		},
		{
			name:     "No index name",
			sql:      "ALTER TABLE users ADD COLUMN email TEXT",
			expected: "",
		},
		{
			name:     "Malformed SQL",
			sql:      "CREATE INDEX",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIndexName(tt.sql)
			if got != tt.expected {
				t.Errorf("extractIndexName(%q) = %q, want %q", tt.sql, got, tt.expected)
			}
		})
	}
}

func TestLockMeasurement_Structure(t *testing.T) {
	// Test that LockMeasurement struct can be created properly
	measurement := &LockMeasurement{
		DurationMS: 100,
		Success:    true,
		Error:      "",
		LockMode:   LockAccessExclusive,
		SQL:        "ALTER TABLE users ADD COLUMN email TEXT",
	}

	if measurement.DurationMS != 100 {
		t.Errorf("DurationMS = %d, want 100", measurement.DurationMS)
	}

	if !measurement.Success {
		t.Error("Expected Success to be true")
	}

	if measurement.LockMode != LockAccessExclusive {
		t.Errorf("LockMode = %v, want %v", measurement.LockMode, LockAccessExclusive)
	}
}

// Integration test - requires PostgreSQL
// Run with: go test -v ./internal/locks/... -run TestMeasureLockDuration_Integration
func TestMeasureLockDuration_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to connect to test database
	connStr := "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Test connection
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
	}

	ctx := context.Background()

	// Clean up test table if it exists
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS test_measurement_table")

	tests := []struct {
		name          string
		step          planner.PlanStep
		shouldSucceed bool
		checkDuration bool
	}{
		{
			name: "CREATE TABLE",
			step: planner.PlanStep{
				Description: "Create test table",
				SQL:         []string{"CREATE TABLE test_measurement_table (id BIGINT PRIMARY KEY, name TEXT)"},
			},
			shouldSucceed: true,
			checkDuration: true,
		},
		{
			name: "ADD COLUMN",
			step: planner.PlanStep{
				Description: "Add column",
				SQL:         []string{"ALTER TABLE test_measurement_table ADD COLUMN email TEXT"},
			},
			shouldSucceed: true,
			checkDuration: true,
		},
		{
			name: "Invalid SQL",
			step: planner.PlanStep{
				Description: "Invalid operation",
				SQL:         []string{"ALTER TABLE nonexistent_table ADD COLUMN x INT"},
			},
			shouldSucceed: false,
			checkDuration: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			measurement, err := MeasureLockDuration(ctx, db, tt.step)

			if err != nil && tt.shouldSucceed {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if measurement == nil {
				t.Fatal("Expected measurement, got nil")
			}

			if tt.shouldSucceed {
				if !measurement.Success {
					t.Errorf("Expected success, got error: %s", measurement.Error)
				}

				if tt.checkDuration && measurement.DurationMS <= 0 {
					t.Errorf("Expected positive duration, got %d ms", measurement.DurationMS)
				}

				if measurement.LockMode == LockAccessShare {
					t.Error("Expected non-trivial lock mode for DDL operation")
				}
			} else {
				if measurement.Success {
					t.Error("Expected failure, got success")
				}
			}
		})
	}

	// Clean up
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS test_measurement_table")
}

// Test MeasureStepLockImpact
func TestMeasureStepLockImpact_Structure(t *testing.T) {
	step := planner.PlanStep{
		Description: "Test step",
		SQL:         []string{"ALTER TABLE users ADD COLUMN email TEXT"},
	}

	// Without DB, just test that it returns impact analysis
	impact := AnalyzeLockImpact(step)

	if impact == nil {
		t.Fatal("Expected impact analysis, got nil")
	}

	if impact.LockMode != LockAccessExclusive {
		t.Errorf("LockMode = %v, want %v", impact.LockMode, LockAccessExclusive)
	}

	if !impact.BlocksReads || !impact.BlocksWrites {
		t.Error("Expected ALTER TABLE to block reads and writes")
	}
}

// Test concurrent operation detection
func TestMeasureConcurrentOperation_IndexName(t *testing.T) {
	// Test that we can extract index names for cleanup
	tests := []struct {
		sql          string
		expectedName string
	}{
		{
			sql:          "CREATE INDEX CONCURRENTLY idx_test ON users(email)",
			expectedName: "idx_test",
		},
		{
			sql:          "CREATE UNIQUE INDEX CONCURRENTLY idx_unique_email ON users(email)",
			expectedName: "idx_unique_email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			name := extractIndexName(tt.sql)
			if name != tt.expectedName {
				t.Errorf("extractIndexName() = %q, want %q", name, tt.expectedName)
			}
		})
	}
}
