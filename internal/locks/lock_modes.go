package locks

import "fmt"

// LockMode represents PostgreSQL lock modes
// See: https://www.postgresql.org/docs/current/explicit-locking.html
type LockMode int

const (
	// LockAccessShare - Acquired by SELECT queries
	// Conflicts only with ACCESS EXCLUSIVE
	LockAccessShare LockMode = iota

	// LockRowShare - Acquired by SELECT FOR UPDATE/FOR SHARE
	// Conflicts with EXCLUSIVE and ACCESS EXCLUSIVE
	LockRowShare

	// LockRowExclusive - Acquired by INSERT, UPDATE, DELETE
	// Conflicts with SHARE, SHARE ROW EXCLUSIVE, EXCLUSIVE, ACCESS EXCLUSIVE
	LockRowExclusive

	// LockShareUpdateExclusive - Acquired by VACUUM, CREATE INDEX CONCURRENTLY
	// Conflicts with SHARE UPDATE EXCLUSIVE, SHARE, SHARE ROW EXCLUSIVE, EXCLUSIVE, ACCESS EXCLUSIVE
	// Key property: Allows concurrent reads AND writes
	LockShareUpdateExclusive

	// LockShare - Acquired by CREATE INDEX (non-concurrent)
	// Conflicts with ROW EXCLUSIVE and above
	// Blocks writes but allows reads
	LockShare

	// LockShareRowExclusive - Rare, not used by standard DDL
	LockShareRowExclusive

	// LockExclusive - Rarely used
	LockExclusive

	// LockAccessExclusive - Acquired by most DDL (ALTER TABLE, DROP TABLE, etc.)
	// Conflicts with EVERYTHING
	// Blocks all reads and writes
	LockAccessExclusive
)

// String returns the human-readable name of the lock mode
func (l LockMode) String() string {
	switch l {
	case LockAccessShare:
		return "ACCESS SHARE"
	case LockRowShare:
		return "ROW SHARE"
	case LockRowExclusive:
		return "ROW EXCLUSIVE"
	case LockShareUpdateExclusive:
		return "SHARE UPDATE EXCLUSIVE"
	case LockShare:
		return "SHARE"
	case LockShareRowExclusive:
		return "SHARE ROW EXCLUSIVE"
	case LockExclusive:
		return "EXCLUSIVE"
	case LockAccessExclusive:
		return "ACCESS EXCLUSIVE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", l)
	}
}

// BlocksReads returns true if this lock mode blocks SELECT queries
func (l LockMode) BlocksReads() bool {
	return l == LockAccessExclusive
}

// BlocksWrites returns true if this lock mode blocks INSERT/UPDATE/DELETE
func (l LockMode) BlocksWrites() bool {
	// SHARE and above block writes
	return l >= LockShare
}

// ImpactLevel returns a simple categorization of the lock's impact
func (l LockMode) ImpactLevel() ImpactLevel {
	switch l {
	case LockAccessShare, LockRowShare, LockRowExclusive:
		return ImpactNone
	case LockShareUpdateExclusive:
		return ImpactLow
	case LockShare:
		return ImpactMedium
	case LockShareRowExclusive, LockExclusive, LockAccessExclusive:
		return ImpactHigh
	default:
		return ImpactHigh
	}
}

// ImpactLevel categorizes the severity of lock impact
type ImpactLevel int

const (
	ImpactNone   ImpactLevel = iota // Normal operations, no blocking
	ImpactLow                       // Minimal blocking (e.g., CONCURRENTLY operations)
	ImpactMedium                    // Blocks writes, allows reads
	ImpactHigh                      // Blocks everything or has severe impact
)

// String returns the human-readable impact level
func (i ImpactLevel) String() string {
	switch i {
	case ImpactNone:
		return "NONE"
	case ImpactLow:
		return "LOW"
	case ImpactMedium:
		return "MEDIUM"
	case ImpactHigh:
		return "HIGH"
	default:
		return "UNKNOWN"
	}
}

// Emoji returns an emoji representing the impact level
func (i ImpactLevel) Emoji() string {
	switch i {
	case ImpactNone:
		return "✓"
	case ImpactLow:
		return "⚡"
	case ImpactMedium:
		return "⚠️"
	case ImpactHigh:
		return "🔴"
	default:
		return "❓"
	}
}

// LockImpact describes the lock impact of a database operation
type LockImpact struct {
	// Operation description
	Operation string

	// Lock mode acquired
	LockMode LockMode

	// Whether the operation blocks reads
	BlocksReads bool

	// Whether the operation blocks writes
	BlocksWrites bool

	// Impact level categorization
	Impact ImpactLevel

	// Estimated duration (if measured on shadow DB)
	// Zero value means not measured
	EstimatedDurationMS int64

	// Whether duration was measured on shadow DB
	MeasuredOnShadowDB bool

	// Explanation of why this lock is needed
	Explanation string
}

// IsHighImpact returns true if this operation has high lock impact
func (li *LockImpact) IsHighImpact() bool {
	return li.Impact >= ImpactMedium
}

// RequiresSaferAlternative returns true if a safer alternative should be suggested
func (li *LockImpact) RequiresSaferAlternative() bool {
	// High impact operations should have alternatives suggested
	// OR operations that hold locks for more than 1 second
	return li.IsHighImpact() || li.EstimatedDurationMS > 1000
}
