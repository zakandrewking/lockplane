package planner

import (
	"context"
	"database/sql"

	"github.com/lockplane/lockplane/internal/locks"
)

// DetectLockMode analyzes a plan step and returns the lock mode it will acquire
func DetectLockMode(step PlanStep) locks.LockMode {
	if len(step.SQL) == 0 {
		return locks.LockAccessShare // No SQL = no locks
	}
	// Use the first SQL statement
	return locks.DetectLockModeFromSQL(step.SQL[0])
}

// IsCreateIndexConcurrently returns true if the step creates an index concurrently
func IsCreateIndexConcurrently(step PlanStep) bool {
	if len(step.SQL) == 0 {
		return false
	}
	return locks.IsCreateIndexConcurrentlySQL(step.SQL[0])
}

// IsAddConstraintNotValid returns true if the step adds a constraint with NOT VALID
func IsAddConstraintNotValid(step PlanStep) bool {
	if len(step.SQL) == 0 {
		return false
	}
	return locks.IsAddConstraintNotValidSQL(step.SQL[0])
}

// IsValidateConstraint returns true if the step validates a constraint
func IsValidateConstraint(step PlanStep) bool {
	if len(step.SQL) == 0 {
		return false
	}
	return locks.IsValidateConstraintSQL(step.SQL[0])
}

// MeasureLockDuration measures how long a plan step holds locks on the shadow DB
func MeasureLockDuration(ctx context.Context, db *sql.DB, step PlanStep) (*locks.LockMeasurement, error) {
	return locks.MeasureLockDurationSQL(ctx, db, step.SQL)
}

// GenerateSaferRewrite analyzes a plan step and suggests lock-safe rewrites
func GenerateSaferRewrite(step PlanStep) *locks.SaferRewrite {
	if len(step.SQL) == 0 {
		return nil
	}
	return locks.GenerateSaferRewriteSQL(step.SQL[0], step.Description)
}

// PopulateLockAnalysis analyzes lock impact and populates the plan step's lock metadata
func PopulateLockAnalysis(step *PlanStep) {
	if len(step.SQL) == 0 {
		return
	}

	// Analyze the most impactful SQL statement
	mostImpactfulSQL := step.SQL[0]
	var maxImpact locks.ImpactLevel = locks.ImpactNone
	var maxLockMode locks.LockMode

	for _, sql := range step.SQL {
		lockMode := locks.DetectLockModeFromSQL(sql)
		impact := lockMode.ImpactLevel()
		if impact > maxImpact {
			maxImpact = impact
			maxLockMode = lockMode
			mostImpactfulSQL = sql
		}
	}

	// Populate lock metadata
	step.LockMode = maxLockMode.String()
	step.BlocksReads = maxLockMode.BlocksReads()
	step.BlocksWrites = maxLockMode.BlocksWrites()
	step.LockImpact = generateLockImpactDescription(maxLockMode)
	step.Rewritable = locks.ShouldRewriteSQL(mostImpactfulSQL)
}

// generateLockImpactDescription creates a human-readable description of lock impact
func generateLockImpactDescription(lockMode locks.LockMode) string {
	switch lockMode.ImpactLevel() {
	case locks.ImpactNone:
		return "No blocking (normal database operations continue)"
	case locks.ImpactLow:
		return "Minimal blocking (concurrent reads and writes allowed)"
	case locks.ImpactMedium:
		return "Blocks writes (SELECT queries continue, INSERT/UPDATE/DELETE blocked)"
	case locks.ImpactHigh:
		return "Blocks all access (all queries blocked until complete)"
	default:
		return "Unknown impact"
	}
}

// AnalyzePlan populates lock analysis for all steps in a plan
func AnalyzePlan(plan *Plan) {
	for i := range plan.Steps {
		PopulateLockAnalysis(&plan.Steps[i])
	}
}

// AnalyzeLockImpact returns detailed lock impact information for a plan step
func AnalyzeLockImpact(step PlanStep) *locks.LockImpact {
	lockMode := DetectLockMode(step)

	impact := &locks.LockImpact{
		Operation:    step.Description,
		LockMode:     lockMode,
		BlocksReads:  lockMode.BlocksReads(),
		BlocksWrites: lockMode.BlocksWrites(),
		Impact:       lockMode.ImpactLevel(),
		Explanation:  explainLockMode(step, lockMode),
	}

	return impact
}

// explainLockMode provides a human-readable explanation of why this lock is needed
func explainLockMode(step PlanStep, mode locks.LockMode) string {
	if len(step.SQL) == 0 {
		return "No SQL operations"
	}

	sql := step.SQL[0]
	return locks.ExplainLockModeFromSQL(sql, mode)
}

// MeasureStepLockImpact measures a step and returns enriched LockImpact
func MeasureStepLockImpact(ctx context.Context, db *sql.DB, step PlanStep) (*locks.LockImpact, error) {
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
