package locks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lockplane/lockplane/internal/planner"
)

// SaferRewrite represents a lock-safe alternative to a DDL operation
type SaferRewrite struct {
	// Description of what the rewrite does
	Description string

	// Rewritten SQL (may be multiple statements)
	SQL []string

	// Lock mode of the rewritten operation
	LockMode LockMode

	// Estimated duration (will be measured on shadow DB)
	EstimatedDurationMS int64

	// Tradeoffs of using this rewrite
	Tradeoffs []string

	// Whether this requires multiple steps/phases
	RequiresMultipleSteps bool

	// Notes about usage
	Notes string
}

// GenerateSaferRewrite attempts to generate a lock-safe rewrite for a plan step
// Returns nil if no safer alternative exists
func GenerateSaferRewrite(step planner.PlanStep) *SaferRewrite {
	if len(step.SQL) == 0 {
		return nil
	}

	sql := strings.TrimSpace(step.SQL[0])
	if sql == "" {
		return nil
	}

	sqlUpper := strings.ToUpper(sql)

	// Pattern 1: CREATE INDEX → CREATE INDEX CONCURRENTLY
	if rewrite := rewriteCreateIndex(sql, sqlUpper); rewrite != nil {
		return rewrite
	}

	// Pattern 2: ADD CONSTRAINT → ADD CONSTRAINT NOT VALID + VALIDATE
	if rewrite := rewriteAddConstraint(sql, sqlUpper, step.Description); rewrite != nil {
		return rewrite
	}

	// Pattern 3: ALTER COLUMN TYPE → Multi-phase suggestion
	if rewrite := suggestMultiPhaseForAlterType(sql, sqlUpper); rewrite != nil {
		return rewrite
	}

	return nil
}

// rewriteCreateIndex converts CREATE INDEX to CREATE INDEX CONCURRENTLY
func rewriteCreateIndex(sql, sqlUpper string) *SaferRewrite {
	// Only rewrite if NOT already CONCURRENTLY
	if strings.Contains(sqlUpper, "CONCURRENTLY") {
		return nil
	}

	// Match CREATE INDEX or CREATE UNIQUE INDEX
	if !strings.HasPrefix(sqlUpper, "CREATE INDEX") && !strings.HasPrefix(sqlUpper, "CREATE UNIQUE INDEX") {
		return nil
	}

	// Insert CONCURRENTLY after CREATE [UNIQUE] INDEX, preserving original case
	var rewrittenSQL string
	if strings.HasPrefix(sqlUpper, "CREATE UNIQUE INDEX") {
		rewrittenSQL = regexp.MustCompile(`(?i)^(CREATE\s+UNIQUE\s+INDEX)`).ReplaceAllString(sql, "$1 CONCURRENTLY")
	} else {
		rewrittenSQL = regexp.MustCompile(`(?i)^(CREATE\s+INDEX)`).ReplaceAllString(sql, "$1 CONCURRENTLY")
	}

	return &SaferRewrite{
		Description: "Use CREATE INDEX CONCURRENTLY to avoid blocking writes",
		SQL:         []string{rewrittenSQL},
		LockMode:    LockShareUpdateExclusive,
		Tradeoffs: []string{
			"Takes longer to build (requires multiple table scans)",
			"Cannot run inside a transaction",
			"May create invalid index if interrupted (must monitor completion)",
			"Allows concurrent INSERT/UPDATE/DELETE during build",
		},
		RequiresMultipleSteps: false,
		Notes:                 "Monitor index creation: SELECT * FROM pg_stat_progress_create_index",
	}
}

// rewriteAddConstraint converts ADD CONSTRAINT to two-phase approach
func rewriteAddConstraint(sql, sqlUpper, description string) *SaferRewrite {
	// Must be ALTER TABLE ADD CONSTRAINT
	if !strings.Contains(sqlUpper, "ALTER TABLE") || !strings.Contains(sqlUpper, "ADD CONSTRAINT") {
		return nil
	}

	// Skip if already using NOT VALID
	if strings.Contains(sqlUpper, "NOT VALID") {
		return nil
	}

	// Skip if this is VALIDATE CONSTRAINT
	if strings.Contains(sqlUpper, "VALIDATE CONSTRAINT") {
		return nil
	}

	// Extract table name and constraint details
	tableName := extractTableName(sql)
	constraintName := extractConstraintName(sql)

	if tableName == "" {
		return nil // Can't rewrite without table name
	}

	// Phase 1: Add constraint with NOT VALID
	phase1SQL := sql
	if !strings.HasSuffix(strings.TrimSpace(sql), ";") {
		phase1SQL += " NOT VALID"
	} else {
		phase1SQL = strings.TrimSuffix(strings.TrimSpace(sql), ";") + " NOT VALID;"
	}

	// Phase 2: Validate constraint
	phase2SQL := fmt.Sprintf("ALTER TABLE %s VALIDATE CONSTRAINT %s", tableName, constraintName)

	return &SaferRewrite{
		Description: "Add constraint in two phases: NOT VALID + VALIDATE to avoid long exclusive lock",
		SQL:         []string{phase1SQL, phase2SQL},
		LockMode:    LockShareUpdateExclusive, // VALIDATE takes this lock
		Tradeoffs: []string{
			"Requires two separate operations",
			"Phase 1: Brief ACCESS EXCLUSIVE lock (~100ms)",
			"Phase 2: Longer SHARE UPDATE EXCLUSIVE lock (allows reads/writes)",
			"New rows validated immediately, existing rows validated in phase 2",
			"Total time is longer but safer for production",
		},
		RequiresMultipleSteps: true,
		Notes:                 "Execute phase 1, then phase 2 after verifying no errors",
	}
}

// suggestMultiPhaseForAlterType suggests multi-phase migration for type changes
func suggestMultiPhaseForAlterType(sql, sqlUpper string) *SaferRewrite {
	// Must be ALTER TABLE ... ALTER COLUMN ... TYPE
	if !strings.Contains(sqlUpper, "ALTER TABLE") ||
		!strings.Contains(sqlUpper, "ALTER COLUMN") ||
		!strings.Contains(sqlUpper, "TYPE") {
		return nil
	}

	// Extract table and column names
	tableName := extractTableName(sql)
	columnName := extractColumnNameFromAlter(sql)

	if tableName == "" || columnName == "" {
		return nil
	}

	return &SaferRewrite{
		Description: "ALTER COLUMN TYPE requires multi-phase migration to avoid downtime",
		SQL:         nil, // No direct SQL rewrite - requires multi-phase plan
		LockMode:    LockShareUpdateExclusive,
		Tradeoffs: []string{
			"Requires 3-5 phases with code deployments",
			"Phase 1: Add new column with new type",
			"Phase 2: Dual-write to both columns (code deploy)",
			"Phase 3: Backfill data to new column",
			"Phase 4: Migrate reads to new column (code deploy)",
			"Phase 5: Drop old column",
			"Temporary storage overhead (two columns exist)",
		},
		RequiresMultipleSteps: true,
		Notes: fmt.Sprintf("Generate multi-phase plan: lockplane plan-multiphase --pattern type_change --table %s --column %s",
			tableName, columnName),
	}
}

// InjectLockTimeout adds lock_timeout to SQL for safety
func InjectLockTimeout(sql string, timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		return sql
	}

	// Remove trailing semicolon if present
	sql = strings.TrimSuffix(strings.TrimSpace(sql), ";")

	// Wrap in transaction with lock_timeout
	return fmt.Sprintf("SET lock_timeout = '%ds'; %s;", timeoutSeconds, sql)
}

// extractTableName attempts to extract table name from ALTER TABLE statement
func extractTableName(sql string) string {
	// Pattern: ALTER TABLE table_name ...
	re := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractConstraintName attempts to extract constraint name from ADD CONSTRAINT
func extractConstraintName(sql string) string {
	// Pattern: ADD CONSTRAINT constraint_name ...
	// Must not match CHECK, UNIQUE, FOREIGN, PRIMARY as those are keywords not names
	re := regexp.MustCompile(`(?i)ADD\s+CONSTRAINT\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		constraintName := matches[1]
		// Don't treat constraint type keywords as names
		upperName := strings.ToUpper(constraintName)
		if upperName != "CHECK" && upperName != "UNIQUE" && upperName != "FOREIGN" && upperName != "PRIMARY" {
			return constraintName
		}
	}

	// If no explicit name, generate one based on table name
	tableName := extractTableName(sql)
	if tableName != "" {
		// Simple default constraint name
		if strings.Contains(strings.ToUpper(sql), "CHECK") {
			return tableName + "_check"
		}
		if strings.Contains(strings.ToUpper(sql), "UNIQUE") {
			return tableName + "_unique"
		}
		if strings.Contains(strings.ToUpper(sql), "FOREIGN KEY") {
			return tableName + "_fkey"
		}
	}

	return "constraint_name"
}

// extractColumnNameFromAlter attempts to extract column name from ALTER COLUMN
func extractColumnNameFromAlter(sql string) string {
	// Pattern: ALTER COLUMN column_name TYPE ...
	re := regexp.MustCompile(`(?i)ALTER\s+COLUMN\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ShouldRewrite determines if a step should be rewritten based on impact
func ShouldRewrite(impact *LockImpact) bool {
	// Rewrite if:
	// 1. High impact operation
	// 2. OR operation that holds locks for more than 1 second
	// 3. OR operation that blocks writes
	return impact.IsHighImpact() ||
		impact.EstimatedDurationMS > 1000 ||
		impact.BlocksWrites
}
