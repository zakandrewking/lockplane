# Lock Analysis and Safe DDL Rewrites

**Status**: 🏗️ In Progress (Phase 3 Complete)
**Created**: 2025-11-14
**Goal**: Analyze DDL lock impact and provide lock-safe rewrites to avoid downtime

---

## Progress Checklist

### Phase 1: Research & Design ✅
- [x] Review PostgreSQL lock modes and their implications
- [x] Research lock-safe rewrite patterns
- [x] Design lock analysis architecture
- [x] Plan shadow DB measurement approach
- [x] Document lock types and operations
- [x] Create implementation plan

### Phase 2: Lock Type Definitions ✅
- [x] Create `internal/locks/` package
- [x] Define PostgreSQL lock modes (enum or constants)
- [x] Map DDL operations to lock modes
- [x] Add lock level metadata to each operation
- [x] Add impact level categorization
- [x] Comprehensive test coverage (13 test functions)
- [ ] Add SQLite lock handling (table-level locks) - Future enhancement

### Phase 3: Lock-Safe Rewrites ✅
- [x] Implement CREATE INDEX → CREATE INDEX CONCURRENTLY
- [x] Implement ADD CONSTRAINT → ADD CONSTRAINT NOT VALID + VALIDATE
- [x] Implement ALTER COLUMN TYPE → multi-phase suggestion
- [x] Add lock_timeout injection for safety
- [x] Generate warnings for lock-heavy operations (ShouldRewrite function)
- [x] Comprehensive test coverage (11 test functions, 25+ sub-tests)
- [ ] Add configuration for rewrite preferences - Future enhancement

### Phase 4: Shadow DB Lock Measurement 🏗️
- [ ] Implement lock timing on shadow DB
- [ ] Measure DDL execution duration
- [ ] Capture lock wait times
- [ ] Test with realistic data sizes
- [ ] Report: "Will hold lock for ~X seconds"
- [ ] Compare before/after with rewrites

### Phase 5: Lock Impact Reporting 🏗️
- [ ] Add lock analysis to validation reports
- [ ] Show lock type for each operation
- [ ] Estimate blocked queries based on lock duration
- [ ] Visual lock impact display
- [ ] Suggest alternatives for high-impact operations
- [ ] Add to plan JSON output

### Phase 6: Integration & Testing 🏗️
- [ ] Unit tests for lock type detection
- [ ] Tests for rewrite generation
- [ ] Integration tests with shadow DB
- [ ] Test lock measurement accuracy
- [ ] Performance benchmarks
- [ ] End-to-end workflow tests

### Phase 7: Documentation 📚
- [ ] Lock analysis guide
- [ ] Lock-safe patterns documentation
- [ ] Update README with lock analysis examples
- [ ] Add troubleshooting guide for lock issues
- [ ] Document configuration options

---

## Context

### The Problem

DDL operations in PostgreSQL take locks that can block reads and writes, causing:
- **Application timeouts** - Queries wait for locks, requests pile up
- **Cascading failures** - Blocked connections exhaust connection pools
- **Downtime** - Long-running DDL blocks all traffic
- **Production incidents** - "Simple" migrations cause outages

**Common scenarios:**
```sql
-- This ALTER TABLE takes an AccessExclusive lock
-- Blocks ALL reads and writes until complete
ALTER TABLE users ADD COLUMN last_seen TIMESTAMP;

-- On a 10M row table, this might hold the lock for 30+ seconds
-- During that time: 0 queries succeed, all requests timeout
```

### Why This Matters

Teams avoid schema changes because they're scared of locks. This leads to:
- Technical debt accumulation
- Workarounds instead of proper fixes
- Late-night "maintenance window" deployments
- Resistance to database schema improvements

**Lockplane can solve this by:**
1. **Measuring** lock impact before production (shadow DB)
2. **Reporting** exactly what locks will be taken and for how long
3. **Rewriting** DDL to use lock-safe alternatives automatically
4. **Suggesting** multi-phase approaches when needed

---

## PostgreSQL Lock Modes

PostgreSQL has 8 lock modes with different compatibility rules:

### Lock Modes (Least to Most Restrictive)

1. **ACCESS SHARE** - `SELECT` queries
   - Conflicts with: ACCESS EXCLUSIVE
   - Allows: All other operations

2. **ROW SHARE** - `SELECT FOR UPDATE`
   - Conflicts with: EXCLUSIVE, ACCESS EXCLUSIVE
   - Allows: Most operations

3. **ROW EXCLUSIVE** - `INSERT`, `UPDATE`, `DELETE`
   - Conflicts with: SHARE, SHARE ROW EXCLUSIVE, EXCLUSIVE, ACCESS EXCLUSIVE
   - Allows: SELECT, SELECT FOR UPDATE

4. **SHARE UPDATE EXCLUSIVE** - `VACUUM`, `CREATE INDEX CONCURRENTLY`
   - Conflicts with: SHARE UPDATE EXCLUSIVE, SHARE, SHARE ROW EXCLUSIVE, EXCLUSIVE, ACCESS EXCLUSIVE
   - Allows: SELECT, SELECT FOR UPDATE, INSERT/UPDATE/DELETE

5. **SHARE** - `CREATE INDEX` (non-concurrent)
   - Conflicts with: ROW EXCLUSIVE, SHARE UPDATE EXCLUSIVE, SHARE ROW EXCLUSIVE, EXCLUSIVE, ACCESS EXCLUSIVE
   - Allows: SELECT queries only

6. **SHARE ROW EXCLUSIVE** - Not used by Lockplane operations

7. **EXCLUSIVE** - Rarely used

8. **ACCESS EXCLUSIVE** - Most DDL operations
   - Conflicts with: EVERYTHING
   - Blocks: All reads and writes
   - Used by: ALTER TABLE, DROP TABLE, TRUNCATE, etc.

### Common Operations and Lock Levels

| Operation | Lock Mode | Impact | Duration |
|-----------|-----------|--------|----------|
| `SELECT` | ACCESS SHARE | None | N/A |
| `INSERT/UPDATE/DELETE` | ROW EXCLUSIVE | Normal | N/A |
| `CREATE INDEX` | SHARE | Blocks writes | Long |
| `CREATE INDEX CONCURRENTLY` | SHARE UPDATE EXCLUSIVE | Low impact | Longer |
| `ALTER TABLE ADD COLUMN` | ACCESS EXCLUSIVE | Blocks everything | Fast |
| `ALTER TABLE ADD COLUMN (with default)` | ACCESS EXCLUSIVE | Blocks everything | Slow |
| `ALTER TABLE ALTER COLUMN TYPE` | ACCESS EXCLUSIVE | Blocks everything | Very slow |
| `ADD CONSTRAINT` | ACCESS EXCLUSIVE | Blocks everything | Depends on data |
| `ADD CONSTRAINT NOT VALID` | SHARE UPDATE EXCLUSIVE | Low impact | Fast |
| `VALIDATE CONSTRAINT` | SHARE UPDATE EXCLUSIVE | Low impact | Slow but safe |
| `DROP TABLE` | ACCESS EXCLUSIVE | Blocks everything | Fast |
| `DROP COLUMN` | ACCESS EXCLUSIVE | Blocks everything | Fast |

---

## Lock-Safe Rewrite Patterns

### Pattern 1: CREATE INDEX → CREATE INDEX CONCURRENTLY

**Unsafe:**
```sql
CREATE INDEX idx_users_email ON users(email);
-- Takes SHARE lock, blocks all writes during index build
-- On 10M rows: ~60 seconds of write blocking
```

**Safe rewrite:**
```sql
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
-- Takes SHARE UPDATE EXCLUSIVE lock
-- Allows concurrent writes, takes longer but safer
-- On 10M rows: ~90 seconds but writes continue
```

**Tradeoffs:**
- ✅ Writes continue during index creation
- ✅ No connection pool exhaustion
- ❌ Takes longer (multiple table scans)
- ❌ Can't run in transaction (auto-commits)
- ⚠️ Needs monitoring (can leave invalid index if interrupted)

**When to use:** Always for production, unless:
- Table is small (<1000 rows)
- In a maintenance window
- Explicit user override

### Pattern 2: ADD CONSTRAINT → ADD CONSTRAINT NOT VALID + VALIDATE

**Unsafe:**
```sql
ALTER TABLE orders ADD CONSTRAINT check_positive_amount
  CHECK (amount > 0);
-- Takes ACCESS EXCLUSIVE lock
-- Scans entire table to validate all rows
-- On 10M rows: ~45 seconds blocking everything
```

**Safe rewrite:**
```sql
-- Phase 1: Add constraint without validating existing rows
ALTER TABLE orders ADD CONSTRAINT check_positive_amount
  CHECK (amount > 0) NOT VALID;
-- Takes ACCESS EXCLUSIVE lock briefly (~100ms)
-- Does NOT scan table

-- Phase 2: Validate in background
ALTER TABLE orders VALIDATE CONSTRAINT check_positive_amount;
-- Takes SHARE UPDATE EXCLUSIVE lock
-- Scans table but allows reads/writes
-- On 10M rows: ~60 seconds but queries continue
```

**When to use:** Always for large tables (>10k rows)

### Pattern 3: ALTER COLUMN TYPE → Multi-Phase Migration

**Unsafe:**
```sql
ALTER TABLE users ALTER COLUMN age TYPE INTEGER USING age::INTEGER;
-- Takes ACCESS EXCLUSIVE lock
-- Rewrites entire table
-- On 10M rows: 5+ minutes blocking everything
```

**Safe approach:** Use multi-phase migration
```sql
-- Phase 1: Add new column
ALTER TABLE users ADD COLUMN age_int INTEGER;

-- Phase 2: Backfill in batches (application-level or batched UPDATE)
UPDATE users SET age_int = age::INTEGER WHERE id BETWEEN 1 AND 100000;
-- Repeat in batches...

-- Phase 3: Switch application to use age_int

-- Phase 4: Drop old column
ALTER TABLE users DROP COLUMN age;
```

**When to suggest:**
- Type change requires table rewrite
- Table has >10k rows
- Incompatible type conversion

### Pattern 4: Add lock_timeout Safety

**Inject lock_timeout:**
```sql
SET lock_timeout = '2s';
ALTER TABLE users ADD COLUMN last_seen TIMESTAMP;
```

**Why:**
- Fails fast instead of blocking indefinitely
- Prevents connection pool exhaustion
- Allows retry with better timing

**When to use:** Always, configurable timeout value

---

## Implementation Design

### Architecture

```
Plan Generation
    ↓
Lock Analysis
    ↓
Lock-Safe Rewrites (if enabled)
    ↓
Shadow DB Measurement
    ↓
Lock Impact Report
    ↓
Plan with Lock Metadata
```

### Data Structures

```go
// Lock mode enum
type LockMode int

const (
	LockAccessShare LockMode = iota
	LockRowShare
	LockRowExclusive
	LockShareUpdateExclusive
	LockShare
	LockShareRowExclusive
	LockExclusive
	LockAccessExclusive
)

// Lock impact metadata
type LockImpact struct {
	Operation     string        // "ALTER TABLE users ADD COLUMN email"
	LockMode      LockMode      // LockAccessExclusive
	Duration      time.Duration // Measured on shadow DB
	BlocksReads   bool          // Does it block SELECT?
	BlocksWrites  bool          // Does it block INSERT/UPDATE/DELETE?
	EstimatedQPS  int           // Queries blocked per second (if known)

	// Alternatives
	SaferAlternative *SaferRewrite // nil if none available
}

// Safer rewrite suggestion
type SaferRewrite struct {
	Description   string   // "Use CREATE INDEX CONCURRENTLY"
	SQL           []string // Rewritten SQL
	LockMode      LockMode // Lower lock level
	ExpectedDuration time.Duration // Usually longer but safer
	Tradeoffs     []string // List of tradeoffs
}
```

### Lock Detection Logic

```go
func DetectLockMode(step PlanStep) LockMode {
	// Detect based on operation type
	switch {
	case isCreateIndex(step):
		if step.Concurrent {
			return LockShareUpdateExclusive
		}
		return LockShare

	case isAddConstraint(step):
		if step.NotValid {
			return LockAccessExclusive // Brief
		}
		return LockAccessExclusive // Long

	case isValidateConstraint(step):
		return LockShareUpdateExclusive

	case isAlterTable(step):
		// Most ALTER TABLE operations
		return LockAccessExclusive

	case isDropTable(step):
		return LockAccessExclusive

	default:
		return LockAccessShare
	}
}
```

### Safe Rewrite Generator

```go
func GenerateSafeRewrite(step PlanStep, driver database.Driver) *SaferRewrite {
	switch {
	case isCreateIndex(step) && !step.Concurrent:
		return &SaferRewrite{
			Description: "Use CREATE INDEX CONCURRENTLY to avoid blocking writes",
			SQL: rewriteIndexConcurrently(step),
			LockMode: LockShareUpdateExclusive,
			Tradeoffs: []string{
				"Takes longer to build",
				"Cannot run inside transaction",
				"May create invalid index if interrupted",
			},
		}

	case isAddConstraint(step) && !step.NotValid:
		return &SaferRewrite{
			Description: "Add constraint in two phases: NOT VALID + VALIDATE",
			SQL: []string{
				addConstraintNotValid(step),
				validateConstraint(step),
			},
			LockMode: LockShareUpdateExclusive,
			Tradeoffs: []string{
				"Requires two operations",
				"Total time is longer",
				"New rows validated immediately, old rows validated in phase 2",
			},
		}

	case isAlterColumnType(step):
		return &SaferRewrite{
			Description: "Use multi-phase migration to avoid table rewrite",
			SQL: nil, // Point to multi-phase plan
			LockMode: LockShareUpdateExclusive,
			Tradeoffs: []string{
				"Requires code deployment coordination",
				"Takes multiple phases to complete",
				"Temporary storage overhead (two columns)",
			},
		}

	default:
		return nil
	}
}
```

### Shadow DB Measurement

```go
func MeasureLockImpact(ctx context.Context, db *sql.DB, step PlanStep) (*LockImpact, error) {
	// Execute on shadow DB and measure duration
	start := time.Now()

	// Wrap in transaction with lock monitoring
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Execute the DDL
	_, err = tx.ExecContext(ctx, step.SQL[0])
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("shadow DB execution failed: %w", err)
	}

	lockMode := DetectLockMode(step)

	return &LockImpact{
		Operation: step.Description,
		LockMode: lockMode,
		Duration: duration,
		BlocksReads: blocksReads(lockMode),
		BlocksWrites: blocksWrites(lockMode),
	}, nil
}

func blocksReads(mode LockMode) bool {
	return mode == LockAccessExclusive
}

func blocksWrites(mode LockMode) bool {
	return mode >= LockShare
}
```

---

## Output Examples

### Validation Report with Lock Analysis

```
📋 Migration Plan Validation

✓ Source hash: abc123...
✓ 3 migration steps

Lock Impact Analysis:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 1: Add email column to users table
  Lock: ACCESS EXCLUSIVE (blocks all reads and writes)
  Estimated duration: ~0.2s (measured on shadow DB with 1M rows)
  Impact: LOW - Brief lock, fast operation

Step 2: Create index on users.email
  Lock: SHARE (blocks all writes, allows reads)
  Estimated duration: ~45s (measured on shadow DB with 1M rows)
  Impact: HIGH - Long write blocking

  ⚡ Safer Alternative Available:
     Use CREATE INDEX CONCURRENTLY
     Lock: SHARE UPDATE EXCLUSIVE (allows reads and writes)
     Estimated duration: ~65s (20s longer but safer)

     Apply rewrite: lockplane plan --lock-safe-rewrites

Step 3: Add CHECK constraint on orders.amount
  Lock: ACCESS EXCLUSIVE (blocks all reads and writes)
  Estimated duration: ~30s (measured on shadow DB with 500k rows)
  Impact: HIGH - Validates all existing rows

  ⚡ Safer Alternative Available:
     Add constraint in two phases: NOT VALID + VALIDATE
     Phase 1 lock: ACCESS EXCLUSIVE (~100ms)
     Phase 2 lock: SHARE UPDATE EXCLUSIVE (~35s, allows traffic)

     Apply rewrite: lockplane plan --lock-safe-rewrites

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Summary:
  Total estimated lock time: ~75s
  Operations blocking reads: 2
  Operations blocking writes: 3
  Safer alternatives available: 2

⚠️  HIGH IMPACT - This migration will block traffic for ~75 seconds
    Consider using --lock-safe-rewrites flag
```

### Plan with Lock Metadata (JSON)

```json
{
  "source_hash": "abc123...",
  "steps": [
    {
      "description": "Create index on users.email",
      "sql": ["CREATE INDEX idx_users_email ON users(email)"],
      "lock_impact": {
        "lock_mode": "SHARE",
        "blocks_reads": false,
        "blocks_writes": true,
        "estimated_duration_ms": 45000,
        "measured_on_shadow_db": true
      },
      "safer_alternative": {
        "description": "Use CREATE INDEX CONCURRENTLY",
        "sql": ["CREATE INDEX CONCURRENTLY idx_users_email ON users(email)"],
        "lock_mode": "SHARE UPDATE EXCLUSIVE",
        "estimated_duration_ms": 65000,
        "tradeoffs": [
          "Takes 20s longer",
          "Cannot run in transaction",
          "Allows concurrent writes"
        ]
      }
    }
  ]
}
```

---

## Configuration

### lockplane.toml

```toml
[lock_analysis]
# Enable lock analysis on shadow DB
enabled = true

# Automatically apply lock-safe rewrites
auto_rewrite = false  # Default: require --lock-safe-rewrites flag

# Lock timeout for safety (Postgres)
lock_timeout = "2s"

# Warn if lock duration exceeds threshold
warn_threshold_ms = 1000

# Error if lock duration exceeds threshold
error_threshold_ms = 30000

[lock_analysis.rewrites]
# Control which rewrites are applied
create_index_concurrently = true
add_constraint_not_valid = true
inject_lock_timeout = true
suggest_multiphase = true
```

---

## CLI Integration

### New Flags

```bash
# Generate plan with lock-safe rewrites
lockplane plan --from current.json --to desired.json --lock-safe-rewrites

# Analyze lock impact without applying rewrites
lockplane plan --from current.json --to desired.json --analyze-locks

# Skip lock analysis (faster, less safe)
lockplane plan --from current.json --to desired.json --skip-lock-analysis

# Validate with lock impact report
lockplane validate plan plan.json --show-lock-impact
```

### Example Usage

```bash
# Step 1: Generate plan and see lock impact
lockplane plan --from prod --to schema/ --analyze-locks

# Output shows:
# ⚠️  Step 2 will hold SHARE lock for ~45s
# ⚡ Safer alternative available: CREATE INDEX CONCURRENTLY

# Step 2: Apply rewrites
lockplane plan --from prod --to schema/ --lock-safe-rewrites > safe-plan.json

# Step 3: Validate on shadow DB
lockplane validate plan safe-plan.json --show-lock-impact

# Step 4: Apply to production
lockplane apply safe-plan.json --target prod
```

---

## Testing Strategy

### Unit Tests

- Lock mode detection for all DDL operations
- Safe rewrite generation
- Lock compatibility logic
- Configuration parsing

### Integration Tests

- Shadow DB lock measurement
- Concurrent rewrite execution
- Multi-phase suggestion generation
- End-to-end workflow

### Performance Tests

- Lock duration measurement accuracy
- Shadow DB overhead
- Large table scenarios (1M, 10M rows)

---

## Success Criteria

1. ✅ Accurately detect lock modes for all DDL operations
2. ✅ Measure lock duration on shadow DB within 10% accuracy
3. ✅ Generate valid lock-safe rewrites for common operations
4. ✅ Clear lock impact reports in validation
5. ✅ Configuration options for rewrite preferences
6. ✅ Documentation with real-world examples

---

## Future Enhancements

### Lock Monitoring

- Real-time lock monitoring during migration
- Abort if lock wait exceeds threshold
- Report blocked queries

### Advanced Rewrites

- Partition table migrations
- Large backfill optimization
- Zero-downtime table rewrites

### Lock Visualization

- Show lock compatibility matrix
- Timeline view of lock acquisition
- Dependency graph of blocked operations

---

## References

- PostgreSQL Documentation: [Explicit Locking](https://www.postgresql.org/docs/current/explicit-locking.html)
- [Lock Monitoring Wiki](https://wiki.postgresql.org/wiki/Lock_Monitoring)
- [CREATE INDEX CONCURRENTLY](https://www.postgresql.org/docs/current/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY)
- [NOT VALID Constraints](https://www.postgresql.org/docs/current/sql-altertable.html#SQL-ALTERTABLE-NOTES)
