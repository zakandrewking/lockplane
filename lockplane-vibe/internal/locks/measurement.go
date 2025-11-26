package locks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
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

// MeasureLockDurationSQL measures how long SQL statements hold locks
// by executing them in a transaction on shadow DB and rolling back
func MeasureLockDurationSQL(ctx context.Context, db *sql.DB, sqlStatements []string) (*LockMeasurement, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	if len(sqlStatements) == 0 {
		return &LockMeasurement{
			Success:  false,
			Error:    "no SQL to measure",
			LockMode: LockAccessShare,
		}, nil
	}

	// Detect lock mode for this operation
	sql := sqlStatements[0]
	lockMode := DetectLockModeFromSQL(sql)

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

	duration := time.Since(startTime)

	measurement := &LockMeasurement{
		DurationMS: duration.Milliseconds(),
		Success:    execErr == nil,
		LockMode:   lockMode,
		SQL:        sql,
	}

	if execErr != nil {
		measurement.Error = execErr.Error()
		return measurement, execErr
	}

	return measurement, nil
}
