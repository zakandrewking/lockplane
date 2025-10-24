# Design: SQL Safety Validation

**Status**: Draft
**Created**: 2025-10-24
**Author**: Claude Code

## Overview

Enhance `lockplane validate sql` to detect dangerous SQL patterns that are syntactically valid but operationally risky in production migrations. The goal is to catch problems before they cause downtime, data loss, or blocking issues.

## Principles

1. **Progressive Enhancement** - Start with easy wins, add sophistication over time
2. **Clear Guidance** - Every warning includes why it's dangerous and what to do instead
3. **Severity Levels** - `error` (blocks migration), `warning` (shows concern), `info` (best practice)
4. **Zero False Positives** - Only warn on genuinely risky patterns
5. **Fast Validation** - All checks run in <100ms for typical schemas

## Implementation Phases

Organized by implementation difficulty, easiest first.

---

## Phase 1: Parse-Only Detection (Easiest)

These can be detected purely from AST parsing without additional context.

### 1.1 Data Loss Operations

**Severity**: `error`

Detect and block operations that irreversibly delete data:

```sql
DROP TABLE users;
DROP TABLE users CASCADE;
TRUNCATE TABLE sessions;
DELETE FROM users;  -- no WHERE clause
ALTER TABLE users DROP COLUMN email;
```

**Detection**:
- `DROP TABLE` - Check for `Node_DropStmt` with relation type `OBJECT_TABLE`
- `TRUNCATE` - Check for `Node_TruncateStmt`
- `DELETE` without WHERE - Check `Node_DeleteStmt`, verify `whereClause` is nil
- `DROP COLUMN` - Check `Node_AlterTableStmt` with `AlterTableCmd.subtype == AT_DropColumn`

**Messages**:
```
ERROR: DROP TABLE is destructive and irreversible
  Found: DROP TABLE users
  Impact: Permanently deletes all data in 'users' table
  Recommendation: Use separate DROP migration only after verifying data is safely migrated
  Code: dangerous_drop_table

ERROR: DELETE without WHERE clause deletes all rows
  Found: DELETE FROM users
  Impact: Removes all data from 'users' table
  Recommendation: Add WHERE clause or use TRUNCATE with explicit confirmation
  Code: dangerous_delete_all

ERROR: DROP COLUMN permanently deletes data
  Found: ALTER TABLE users DROP COLUMN email
  Impact: All data in 'email' column is lost and cannot be recovered
  Recommendation: Ensure data is migrated or no longer needed before dropping
  Code: dangerous_drop_column
```

**Implementation**:
```go
// In validateSQLSyntax or new validateDangerousPatterns function
func detectDataLoss(stmt *pg_query.Node) []ValidationIssue {
    // Walk AST looking for:
    // - DropStmt
    // - TruncateStmt
    // - DeleteStmt without whereClause
    // - AlterTableStmt with AT_DropColumn
}
```

**Complexity**: Easy (2-4 hours)

---

### 1.2 Blocking Index Operations

**Severity**: `warning`

Detect index operations that lock tables:

```sql
CREATE INDEX users_email_idx ON users(email);
-- Should be: CREATE INDEX CONCURRENTLY users_email_idx ON users(email);

DROP INDEX users_email_idx;
-- Should be: DROP INDEX CONCURRENTLY users_email_idx;
```

**Detection**:
- `CREATE INDEX` - Check `Node_IndexStmt`, verify `concurrent == false`
- `DROP INDEX` - Check `Node_DropStmt` for OBJECT_INDEX, verify `concurrent == false`

**Messages**:
```
WARNING: CREATE INDEX without CONCURRENTLY locks table during build
  Found: CREATE INDEX users_email_idx ON users(email)
  Impact: Table 'users' is locked for reads/writes during index creation
          On large tables, this can take hours and block all queries
  Recommendation: CREATE INDEX CONCURRENTLY users_email_idx ON users(email)
  Note: CONCURRENTLY requires running outside transaction blocks
  Code: blocking_index_creation

WARNING: DROP INDEX without CONCURRENTLY locks table
  Found: DROP INDEX users_email_idx
  Impact: Table 'users' is locked during index drop
  Recommendation: DROP INDEX CONCURRENTLY users_email_idx
  Code: blocking_index_drop
```

**Complexity**: Easy (1-2 hours)

---

### 1.3 Dangerous System Operations

**Severity**: `error`

Detect operations that affect entire database or system:

```sql
DROP DATABASE myapp;
DROP SCHEMA public CASCADE;
ALTER SYSTEM SET max_connections = 10;
VACUUM FULL users;
```

**Detection**:
- `DROP DATABASE` - Check `Node_DropdbStmt`
- `DROP SCHEMA ... CASCADE` - Check `Node_DropStmt` with OBJECT_SCHEMA and cascade
- `ALTER SYSTEM` - Check `Node_AlterSystemStmt`
- `VACUUM FULL` - Check `Node_VacuumStmt` with `options` containing VACOPT_FULL

**Messages**:
```
ERROR: DROP DATABASE is destructive and affects entire database
  Found: DROP DATABASE myapp
  Impact: Deletes entire database and all data
  Recommendation: This should never be in a migration
  Code: dangerous_drop_database

ERROR: VACUUM FULL locks table and rewrites all data
  Found: VACUUM FULL users
  Impact: Exclusive lock on 'users' table, potentially hours of downtime
  Recommendation: Use regular VACUUM or VACUUM (ANALYZE) instead
  Code: dangerous_vacuum_full
```

**Complexity**: Easy (2 hours)

---

### 1.4 Breaking Schema Changes

**Severity**: `warning`

Detect changes that break existing application code:

```sql
ALTER TABLE users RENAME TO customers;
ALTER TABLE users RENAME COLUMN email TO email_address;
ALTER TABLE users ALTER COLUMN status DROP DEFAULT;
```

**Detection**:
- `RENAME TABLE` - Check `Node_RenameStmt` with OBJECT_TABLE
- `RENAME COLUMN` - Check `Node_RenameStmt` with OBJECT_COLUMN
- `DROP DEFAULT` - Check `AlterTableCmd.subtype == AT_DropDefault`

**Messages**:
```
WARNING: RENAME TABLE breaks all queries referencing 'users'
  Found: ALTER TABLE users RENAME TO customers
  Impact: All application code, views, and functions using 'users' will fail
  Recommendation: Use database views for gradual migration
          CREATE VIEW users AS SELECT * FROM customers;
  Code: breaking_rename_table

WARNING: DROP DEFAULT may break INSERT statements
  Found: ALTER TABLE users ALTER COLUMN status DROP DEFAULT
  Impact: INSERTs without explicit 'status' value will fail
  Recommendation: Ensure application code provides 'status' before migration
  Code: breaking_drop_default
```

**Complexity**: Easy (2-3 hours)

---

## Phase 2: Context-Aware Detection (Medium)

These require understanding relationships between statements or schema state.

### 2.1 NOT NULL Without Default or Backfill

**Severity**: `error`

Detect adding NOT NULL to existing tables without ensuring data compatibility:

```sql
-- Dangerous: existing NULLs will cause migration to fail
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

-- Dangerous: breaks INSERTs without email value
ALTER TABLE users ADD COLUMN email TEXT NOT NULL;

-- Safe: has default
ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT 'unknown@example.com';

-- Safe pattern (multi-step):
-- 1. ALTER TABLE users ADD COLUMN email TEXT;
-- 2. UPDATE users SET email = 'unknown@example.com' WHERE email IS NULL;
-- 3. ALTER TABLE users ALTER COLUMN email SET NOT NULL;
```

**Detection**:
1. For `ADD COLUMN ... NOT NULL`:
   - Check `ColumnDef` with `is_not_null == true`
   - Verify `cooked_default` is present
   - If missing default → ERROR

2. For `ALTER COLUMN SET NOT NULL`:
   - Check `AlterTableCmd.subtype == AT_SetNotNull`
   - Look backward in same file for UPDATE statement on same table/column
   - If no UPDATE found → WARNING (can't prove data is non-null)

**Messages**:
```
ERROR: ADD COLUMN NOT NULL without DEFAULT will fail on existing rows
  Found: ALTER TABLE users ADD COLUMN email TEXT NOT NULL
  Impact: Migration fails if table has any existing rows
  Recommendation: Add a DEFAULT value
          ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT 'unknown@example.com'
  Code: not_null_without_default

WARNING: SET NOT NULL without backfill may fail on existing data
  Found: ALTER TABLE users ALTER COLUMN email SET NOT NULL
  Impact: Migration fails if any rows have NULL in 'email' column
  Recommendation: Backfill NULL values first
          UPDATE users SET email = 'unknown@example.com' WHERE email IS NULL;
          ALTER TABLE users ALTER COLUMN email SET NOT NULL;
  Code: set_not_null_without_backfill
```

**Complexity**: Medium (4-6 hours)

---

### 2.2 Foreign Keys Without Indexes

**Severity**: `warning`

Detect foreign keys added without corresponding index on referencing column:

```sql
-- Dangerous: no index on posts.user_id
ALTER TABLE posts ADD CONSTRAINT posts_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id);

-- Safe pattern:
CREATE INDEX CONCURRENTLY posts_user_id_idx ON posts(user_id);
ALTER TABLE posts ADD CONSTRAINT posts_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id);
```

**Detection**:
1. Find all `ADD CONSTRAINT ... FOREIGN KEY` statements
2. Extract referencing columns (e.g., `user_id`)
3. Search for `CREATE INDEX` on same table including those columns
4. If no index found → WARNING

**Implementation**:
```go
type SchemaState struct {
    Tables map[string]*TableInfo
    Indexes map[string]*IndexInfo
}

// Build state from all statements, then validate
func validateForeignKeyIndexes(state *SchemaState) []ValidationIssue {
    for _, fk := range state.ForeignKeys {
        if !hasIndexOnColumns(state, fk.Table, fk.Columns) {
            // Issue warning
        }
    }
}
```

**Messages**:
```
WARNING: Foreign key without index causes slow queries
  Found: ALTER TABLE posts ADD CONSTRAINT posts_user_id_fkey
         FOREIGN KEY (user_id) REFERENCES users(id)
  Impact: Queries joining posts and users on user_id will be slow
          Deletes/updates to users table will be slow
  Recommendation: Create index first
          CREATE INDEX CONCURRENTLY posts_user_id_idx ON posts(user_id);
  Code: foreign_key_without_index
```

**Complexity**: Medium (6-8 hours)

---

### 2.3 Type Changes That Lose Precision

**Severity**: `error`

Detect type changes that can truncate or lose data:

```sql
-- Loses decimal places
ALTER TABLE products ALTER COLUMN price TYPE INTEGER;  -- DECIMAL → INTEGER

-- Can truncate data
ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(10);  -- VARCHAR(255) → VARCHAR(10)

-- Loses timezone info
ALTER TABLE events ALTER COLUMN created_at TYPE TIMESTAMP;  -- TIMESTAMPTZ → TIMESTAMP
```

**Detection**:

Need type compatibility matrix:

```go
var dangerousTypeChanges = map[string]map[string]string{
    "numeric": {
        "integer": "loses decimal precision",
        "bigint": "loses decimal precision",
    },
    "decimal": {
        "integer": "loses decimal precision",
    },
    "timestamp with time zone": {
        "timestamp": "loses timezone information",
    },
    "varchar": {
        "varchar": "check_length_reduction", // Special case: need to compare lengths
    },
}

func checkTypeChange(oldType, newType string) (bool, string) {
    // Look up in matrix
    // For VARCHAR, extract and compare lengths
}
```

**Messages**:
```
ERROR: Type change from NUMERIC to INTEGER loses decimal precision
  Found: ALTER TABLE products ALTER COLUMN price TYPE INTEGER
  Impact: Values like 19.99 become 19, losing cents
  Recommendation: Keep NUMERIC type or use BIGINT for cents
          ALTER TABLE products ALTER COLUMN price_cents TYPE BIGINT;
          UPDATE products SET price_cents = (price * 100)::BIGINT;
  Code: type_change_loses_precision

ERROR: Reducing VARCHAR length may truncate existing data
  Found: ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(10)
  Impact: Usernames longer than 10 characters will be truncated
  Recommendation: Check max length first
          SELECT MAX(LENGTH(username)) FROM users;
  Code: varchar_length_reduction
```

**Complexity**: Medium (8-10 hours) - requires type system knowledge

---

### 2.4 Constraints Without Validation

**Severity**: `warning`

Detect constraints added without `NOT VALID` that will lock table:

```sql
-- Locks table during constraint check
ALTER TABLE posts ADD CONSTRAINT posts_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id);

-- Safe pattern (Postgres 9.1+)
ALTER TABLE posts ADD CONSTRAINT posts_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;
ALTER TABLE posts VALIDATE CONSTRAINT posts_user_id_fkey;

-- Same for CHECK constraints
ALTER TABLE users ADD CONSTRAINT users_age_check CHECK (age >= 0);
-- Should be:
ALTER TABLE users ADD CONSTRAINT users_age_check CHECK (age >= 0) NOT VALID;
ALTER TABLE users VALIDATE CONSTRAINT users_age_check;
```

**Detection**:
- Check `Constraint` node with `contype IN (CONSTR_FOREIGN, CONSTR_CHECK)`
- Verify `skip_validation == false`
- If false → WARNING

**Messages**:
```
WARNING: Adding constraint without NOT VALID locks table
  Found: ALTER TABLE posts ADD CONSTRAINT posts_user_id_fkey
         FOREIGN KEY (user_id) REFERENCES users(id)
  Impact: Table locked while scanning all rows to verify constraint
  Recommendation: Use two-step process for large tables
          ALTER TABLE posts ADD CONSTRAINT posts_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;
          ALTER TABLE posts VALIDATE CONSTRAINT posts_user_id_fkey;
  Note: NOT VALID requires Postgres 9.1+
  Code: constraint_without_not_valid
```

**Complexity**: Medium (3-4 hours)

---

## Phase 3: Advanced Detection (Hard)

These require database connection or statistical analysis.

### 3.1 Batch Operations on Large Tables

**Severity**: `warning`

Detect operations that should be batched:

```sql
-- Dangerous on large tables
UPDATE users SET migrated = true;
DELETE FROM logs WHERE created_at < NOW() - INTERVAL '1 year';
```

**Detection** (requires table stats):
1. Connect to database and get table row counts
2. If UPDATE/DELETE affects table with >100k rows → WARNING
3. Suggest batching approach

**Messages**:
```
WARNING: Unbounded UPDATE on large table (users: 5M rows)
  Found: UPDATE users SET migrated = true
  Impact: Long transaction holds locks, can cause replication lag
  Recommendation: Batch updates in smaller transactions
          DO $$
          DECLARE
            batch_size INTEGER := 10000;
          BEGIN
            LOOP
              UPDATE users SET migrated = true
              WHERE id IN (
                SELECT id FROM users WHERE migrated IS NULL LIMIT batch_size
              );
              EXIT WHEN NOT FOUND;
              COMMIT;
            END LOOP;
          END $$;
  Code: unbatched_update_large_table
```

**Complexity**: Hard (10-12 hours) - requires DB connection, stats gathering

---

### 3.2 Multiple Indexes in Single Migration

**Severity**: `info`

Detect multiple index creations that could be parallelized:

```sql
-- Locks table 3 times sequentially
CREATE INDEX CONCURRENTLY idx1 ON users(email);
CREATE INDEX CONCURRENTLY idx2 ON users(created_at);
CREATE INDEX CONCURRENTLY idx3 ON users(status);

-- Better: Run in parallel (separate transactions)
```

**Detection**:
1. Count `CREATE INDEX` statements on same table
2. If >1 → INFO suggesting parallelization

**Messages**:
```
INFO: Multiple indexes on same table can be created in parallel
  Found: 3 CREATE INDEX statements on 'users' table
  Recommendation: Create indexes in parallel for faster completion
          Run each CREATE INDEX CONCURRENTLY in separate session/transaction
  Code: parallel_index_opportunity
```

**Complexity**: Medium (4 hours)

---

## Implementation Plan

### Milestone 1: Data Loss Prevention (Week 1)
- [ ] Phase 1.1: Data Loss Operations
- [ ] Phase 1.2: Blocking Index Operations
- [ ] Phase 1.3: Dangerous System Operations
- [ ] Phase 1.4: Breaking Schema Changes

**Deliverable**: Catches ~60% of dangerous patterns with zero false positives

### Milestone 2: Constraint Safety (Week 2)
- [ ] Phase 2.1: NOT NULL Without Default
- [ ] Phase 2.2: Foreign Keys Without Indexes
- [ ] Phase 2.4: Constraints Without Validation

**Deliverable**: Catches ~80% of dangerous patterns

### Milestone 3: Type Safety (Week 3)
- [ ] Phase 2.3: Type Changes That Lose Precision

**Deliverable**: Catches precision loss bugs

### Milestone 4: Performance Optimization (Week 4+)
- [ ] Phase 3.1: Batch Operations
- [ ] Phase 3.2: Multiple Indexes
- [ ] Phase 3.3: Lock duration estimation

**Deliverable**: Performance guidance

---

## File Structure

```
validate_sql.go              # Existing syntax validation
validate_sql_safety.go       # New safety validations (Phase 1-2)
validate_sql_performance.go  # Performance validations (Phase 3)
docs/dangerous-patterns.md   # User documentation with examples
```

---

## Configuration

Allow users to configure validation levels:

```toml
# lockplane.toml
[validation]
# Validation level: "strict" | "recommended" | "minimal"
level = "recommended"

# Disable specific rules
disable = [
  "blocking_index_creation",  # We know our tables are small
  "foreign_key_without_index", # We add indexes separately
]

# Enable database connection for advanced checks
enable_db_stats = true
```

---

## Testing Strategy

### Unit Tests
```go
func TestDetectDropTable(t *testing.T) {
    sql := "DROP TABLE users;"
    issues := validateDangerousPatterns(sql)
    assert.Len(t, issues, 1)
    assert.Equal(t, "dangerous_drop_table", issues[0].Code)
}
```

### Integration Tests
```go
func TestValidateSQLSafety_RealSchemas(t *testing.T) {
    // Test against examples/dangerous/ directory
    files := glob("examples/dangerous/*.lp.sql")
    for _, file := range files {
        issues := runValidateSQLSafety(file)
        // Verify expected warnings
    }
}
```

### Test Fixtures
Create `examples/dangerous/` with:
- `drop_operations.lp.sql` - DROP TABLE, DROP COLUMN
- `blocking_indexes.lp.sql` - CREATE INDEX without CONCURRENTLY
- `not_null_unsafe.lp.sql` - SET NOT NULL without backfill
- etc.

---

## Success Metrics

1. **Detection Rate**: Catch 80%+ of dangerous patterns in real migrations
2. **False Positive Rate**: <5% false warnings
3. **Performance**: Validation completes in <100ms for typical schemas
4. **Adoption**: 50%+ of users enable safety validation within 3 months

---

## Future Enhancements

1. **Auto-fix Suggestions**: Generate safe alternative SQL
2. **Database-Specific Rules**: MySQL, SQLite-specific patterns
3. **Custom Rules**: User-defined validation rules in Lua/JavaScript
4. **Severity Tuning**: ML-based severity based on table size/usage
5. **Integration**: CI/CD hooks that fail on ERROR-level issues

---

## References

- [Postgres Locking](https://www.postgresql.org/docs/current/explicit-locking.html)
- [Zero-Downtime Migrations](https://www.braintreepayments.com/blog/safe-operations-for-high-volume-postgresql/)
- [Strong Migrations (Ruby)](https://github.com/ankane/strong_migrations)
- [pg_query_go AST Reference](https://github.com/pganalyze/pg_query_go)
