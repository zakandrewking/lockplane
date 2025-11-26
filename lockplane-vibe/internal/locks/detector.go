package locks

import (
	"strings"
)

// DetectLockModeFromSQL analyzes a SQL statement and returns the lock mode it will acquire
// This version doesn't depend on planner types to avoid circular dependencies
func DetectLockModeFromSQL(sql string) LockMode {
	sql = strings.TrimSpace(sql)
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

// IsCreateIndexConcurrentlySQL returns true if the SQL creates an index concurrently
func IsCreateIndexConcurrentlySQL(sql string) bool {
	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(sqlUpper, "CREATE INDEX CONCURRENTLY") ||
		strings.HasPrefix(sqlUpper, "CREATE UNIQUE INDEX CONCURRENTLY")
}

// IsAddConstraintNotValidSQL returns true if the SQL adds a constraint with NOT VALID
func IsAddConstraintNotValidSQL(sql string) bool {
	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.Contains(sqlUpper, "ADD CONSTRAINT") && strings.Contains(sqlUpper, "NOT VALID")
}

// IsValidateConstraintSQL returns true if the SQL validates a constraint
func IsValidateConstraintSQL(sql string) bool {
	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.Contains(sqlUpper, "VALIDATE CONSTRAINT")
}

// ExplainLockModeFromSQL provides a human-readable explanation of why this lock is needed
func ExplainLockModeFromSQL(sql string, mode LockMode) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return "No SQL operations"
	}

	sqlUpper := strings.ToUpper(sql)

	switch mode {
	case LockAccessExclusive:
		if strings.Contains(sqlUpper, "ALTER TABLE") {
			if strings.Contains(sqlUpper, "ADD COLUMN") {
				if strings.Contains(sqlUpper, "DEFAULT") {
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
