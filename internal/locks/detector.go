package locks

import (
	"strings"

	"github.com/lockplane/lockplane/internal/planner"
)

// DetectLockMode analyzes a plan step and returns the lock mode it will acquire
func DetectLockMode(step planner.PlanStep) LockMode {
	if len(step.SQL) == 0 {
		return LockAccessShare // No SQL = no locks
	}

	// Analyze the first SQL statement to determine operation type
	sql := strings.TrimSpace(step.SQL[0])
	if sql == "" {
		return LockAccessShare // Empty SQL = no locks
	}
	sqlUpper := strings.ToUpper(sql)

	// CREATE INDEX patterns
	if strings.HasPrefix(sqlUpper, "CREATE INDEX") || strings.HasPrefix(sqlUpper, "CREATE UNIQUE INDEX") {
		if strings.Contains(sqlUpper, "CONCURRENTLY") {
			return LockShareUpdateExclusive
		}
		return LockShare
	}

	// ALTER TABLE patterns
	if strings.HasPrefix(sqlUpper, "ALTER TABLE") {
		// ADD CONSTRAINT patterns
		if strings.Contains(sqlUpper, "ADD CONSTRAINT") {
			if strings.Contains(sqlUpper, "NOT VALID") {
				// ADD CONSTRAINT NOT VALID is brief ACCESS EXCLUSIVE
				// followed by VALIDATE CONSTRAINT which is SHARE UPDATE EXCLUSIVE
				return LockAccessExclusive
			}
			// Regular ADD CONSTRAINT scans entire table
			return LockAccessExclusive
		}

		// VALIDATE CONSTRAINT - lower lock mode
		if strings.Contains(sqlUpper, "VALIDATE CONSTRAINT") {
			return LockShareUpdateExclusive
		}

		// Most ALTER TABLE operations take ACCESS EXCLUSIVE
		return LockAccessExclusive
	}

	// DROP TABLE, DROP INDEX, TRUNCATE
	if strings.HasPrefix(sqlUpper, "DROP TABLE") ||
		strings.HasPrefix(sqlUpper, "DROP INDEX") ||
		strings.HasPrefix(sqlUpper, "TRUNCATE") {
		return LockAccessExclusive
	}

	// CREATE TABLE - no lock on the table itself (it doesn't exist yet)
	if strings.HasPrefix(sqlUpper, "CREATE TABLE") {
		return LockAccessShare
	}

	// INSERT, UPDATE, DELETE
	if strings.HasPrefix(sqlUpper, "INSERT") ||
		strings.HasPrefix(sqlUpper, "UPDATE") ||
		strings.HasPrefix(sqlUpper, "DELETE") {
		return LockRowExclusive
	}

	// SELECT
	if strings.HasPrefix(sqlUpper, "SELECT") {
		return LockAccessShare
	}

	// Default: assume high lock for safety
	return LockAccessExclusive
}

// AnalyzeLockImpact returns detailed lock impact information for a plan step
func AnalyzeLockImpact(step planner.PlanStep) *LockImpact {
	lockMode := DetectLockMode(step)

	impact := &LockImpact{
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
func explainLockMode(step planner.PlanStep, mode LockMode) string {
	if len(step.SQL) == 0 {
		return "No SQL operations"
	}

	sql := strings.TrimSpace(step.SQL[0])
	sqlUpper := strings.ToUpper(sql)

	switch mode {
	case LockAccessExclusive:
		if strings.Contains(sqlUpper, "ALTER TABLE") {
			if strings.Contains(sqlUpper, "ADD COLUMN") {
				if containsDefault(sqlUpper) {
					return "ALTER TABLE ADD COLUMN with DEFAULT requires rewriting the entire table"
				}
				return "ALTER TABLE requires exclusive access to modify table structure"
			}
			if strings.Contains(sqlUpper, "DROP COLUMN") {
				return "DROP COLUMN requires exclusive access to modify table structure"
			}
			if strings.Contains(sqlUpper, "ALTER COLUMN TYPE") {
				return "Changing column type may require rewriting the entire table"
			}
			if strings.Contains(sqlUpper, "ADD CONSTRAINT") && !strings.Contains(sqlUpper, "NOT VALID") {
				return "ADD CONSTRAINT scans all existing rows to validate the constraint"
			}
			return "ALTER TABLE operation requires exclusive access"
		}
		if strings.Contains(sqlUpper, "DROP TABLE") {
			return "DROP TABLE requires exclusive access to remove the table"
		}
		if strings.Contains(sqlUpper, "TRUNCATE") {
			return "TRUNCATE requires exclusive access to delete all rows"
		}
		return "This operation requires exclusive table access"

	case LockShare:
		if strings.Contains(sqlUpper, "CREATE INDEX") && !strings.Contains(sqlUpper, "CONCURRENTLY") {
			return "CREATE INDEX requires SHARE lock, blocking writes during index build"
		}
		return "This operation blocks writes but allows reads"

	case LockShareUpdateExclusive:
		if strings.Contains(sqlUpper, "CREATE INDEX CONCURRENTLY") {
			return "CREATE INDEX CONCURRENTLY allows concurrent reads and writes"
		}
		if strings.Contains(sqlUpper, "VALIDATE CONSTRAINT") {
			return "VALIDATE CONSTRAINT allows concurrent reads and writes"
		}
		return "This operation allows concurrent reads and writes"

	case LockRowExclusive:
		return "Normal DML operation (INSERT/UPDATE/DELETE)"

	case LockAccessShare:
		return "Read-only operation"

	default:
		return "Standard locking for this operation type"
	}
}

// containsDefault checks if SQL contains DEFAULT clause
func containsDefault(sql string) bool {
	return strings.Contains(sql, "DEFAULT")
}

// IsCreateIndexConcurrently returns true if the step creates an index concurrently
func IsCreateIndexConcurrently(step planner.PlanStep) bool {
	if len(step.SQL) == 0 {
		return false
	}
	sqlUpper := strings.ToUpper(step.SQL[0])
	return strings.HasPrefix(sqlUpper, "CREATE INDEX CONCURRENTLY") ||
		strings.HasPrefix(sqlUpper, "CREATE UNIQUE INDEX CONCURRENTLY")
}

// IsAddConstraintNotValid returns true if the step adds a constraint with NOT VALID
func IsAddConstraintNotValid(step planner.PlanStep) bool {
	if len(step.SQL) == 0 {
		return false
	}
	sqlUpper := strings.ToUpper(step.SQL[0])
	return strings.Contains(sqlUpper, "ADD CONSTRAINT") && strings.Contains(sqlUpper, "NOT VALID")
}

// IsValidateConstraint returns true if the step validates a constraint
func IsValidateConstraint(step planner.PlanStep) bool {
	if len(step.SQL) == 0 {
		return false
	}
	sqlUpper := strings.ToUpper(step.SQL[0])
	return strings.Contains(sqlUpper, "VALIDATE CONSTRAINT")
}
