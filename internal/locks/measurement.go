package locks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lockplane/lockplane/internal/planner"
)

// LockMeasurement represents a measured lock operation on shadow DB
type LockMeasurement struct {
	// Duration of the operation in milliseconds
	DurationMS int64

	// Whether measurement succeeded
	Success bool

	// Error message if measurement failed
	Error string

	// Lock mode detected
	LockMode LockMode

	// SQL that was measured
	SQL string
}

// MeasureLockDuration measures how long a DDL operation holds locks
// by executing it in a transaction on shadow DB and rolling back
func MeasureLockDuration(ctx context.Context, db *sql.DB, step planner.PlanStep) (*LockMeasurement, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	if len(step.SQL) == 0 {
		return &LockMeasurement{
			Success:  false,
			Error:    "no SQL to measure",
			LockMode: LockAccessShare,
		}, nil
	}

	// Detect lock mode for this operation
	lockMode := DetectLockMode(step)

	// Get the SQL to execute
	sql := step.SQL[0]
	if strings.TrimSpace(sql) == "" {
		return &LockMeasurement{
			Success:  false,
			Error:    "empty SQL",
			LockMode: lockMode,
			SQL:      sql,
		}, nil
	}

	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return &LockMeasurement{
			Success:  false,
			Error:    fmt.Sprintf("failed to begin transaction: %v", err),
			LockMode: lockMode,
			SQL:      sql,
		}, err
	}

	// Always rollback to avoid permanent changes
	defer func() {
		_ = tx.Rollback()
	}()

	// Measure execution time
	startTime := time.Now()

	// Execute the SQL
	_, execErr := tx.ExecContext(ctx, sql)

	// Calculate duration
	duration := time.Since(startTime)
	durationMS := duration.Milliseconds()

	if execErr != nil {
		// Check if it's a concurrency-related error we can ignore
		errMsg := execErr.Error()
		if strings.Contains(errMsg, "CONCURRENTLY") && strings.Contains(errMsg, "cannot run inside a transaction") {
			// CREATE INDEX CONCURRENTLY cannot run in transaction
			// Measure it outside transaction
			return measureConcurrentOperation(ctx, db, sql, lockMode)
		}

		return &LockMeasurement{
			Success:    false,
			Error:      fmt.Sprintf("execution failed: %v", execErr),
			LockMode:   lockMode,
			SQL:        sql,
			DurationMS: durationMS,
		}, nil
	}

	return &LockMeasurement{
		Success:    true,
		DurationMS: durationMS,
		LockMode:   lockMode,
		SQL:        sql,
	}, nil
}

// measureConcurrentOperation measures operations that cannot run in transactions
// like CREATE INDEX CONCURRENTLY
func measureConcurrentOperation(ctx context.Context, db *sql.DB, sql string, lockMode LockMode) (*LockMeasurement, error) {
	// For CONCURRENT operations, we need to execute them outside a transaction
	// Since we're on shadow DB, it's safe to actually execute and then clean up

	// Extract index name from SQL to drop it after
	indexName := extractIndexName(sql)

	startTime := time.Now()

	// Execute the operation
	_, execErr := db.ExecContext(ctx, sql)

	duration := time.Since(startTime)
	durationMS := duration.Milliseconds()

	if execErr != nil {
		return &LockMeasurement{
			Success:    false,
			Error:      fmt.Sprintf("concurrent operation failed: %v", execErr),
			LockMode:   lockMode,
			SQL:        sql,
			DurationMS: durationMS,
		}, nil
	}

	// Clean up: drop the created index
	if indexName != "" {
		dropSQL := fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName)
		_, _ = db.ExecContext(ctx, dropSQL)
	}

	return &LockMeasurement{
		Success:    true,
		DurationMS: durationMS,
		LockMode:   lockMode,
		SQL:        sql,
	}, nil
}

// extractIndexName attempts to extract the index name from CREATE INDEX statement
func extractIndexName(sql string) string {
	// Pattern: CREATE [UNIQUE] INDEX [CONCURRENTLY] index_name ON ...
	sqlUpper := strings.ToUpper(sql)

	// Find position of "INDEX"
	indexPos := strings.Index(sqlUpper, "INDEX")
	if indexPos == -1 {
		return ""
	}

	// Find position of "ON"
	onPos := strings.Index(sqlUpper, " ON ")
	if onPos == -1 || onPos <= indexPos {
		return ""
	}

	// Extract the part between "INDEX" and "ON"
	betweenPart := strings.TrimSpace(sql[indexPos+5 : onPos])

	// Remove "CONCURRENTLY" if present (case-insensitive)
	betweenPart = strings.TrimSpace(strings.Replace(
		betweenPart,
		"CONCURRENTLY",
		"",
		1,
	))
	betweenPart = strings.TrimSpace(strings.Replace(
		betweenPart,
		"concurrently",
		"",
		1,
	))

	// The remaining part should be the index name
	words := strings.Fields(betweenPart)
	if len(words) > 0 {
		return words[0]
	}

	return ""
}

// MeasureStepLockImpact measures a step and returns enriched LockImpact
func MeasureStepLockImpact(ctx context.Context, db *sql.DB, step planner.PlanStep) (*LockImpact, error) {
	// Analyze lock impact
	impact := AnalyzeLockImpact(step)

	// Measure duration on shadow DB
	measurement, err := MeasureLockDuration(ctx, db, step)
	if err != nil {
		// Return impact without measurement if measurement failed
		return impact, err
	}

	// Enrich impact with measured duration
	if measurement.Success {
		impact.EstimatedDurationMS = measurement.DurationMS
		impact.MeasuredOnShadowDB = true
	}

	return impact, nil
}
