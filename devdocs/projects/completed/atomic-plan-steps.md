# Atomic Plan Steps with Multiple SQL Operations

## Progress Checklist

- [x] Phase 1: Design and Schema Updates
  - [x] Update PlanStep structure to support SQL array
  - [x] Update JSON schema for plans
  - [x] Update plan validation
- [x] Phase 2: Core Implementation
  - [x] Update planner to use new step structure
  - [x] Update SQLite generator methods
  - [x] Update Postgres generator (verify single-statement compatibility)
  - [x] Update plan loader/writer
- [x] Phase 3: Execution Engine Updates
  - [x] Update applyPlan to handle SQL arrays
  - [x] Update dryRunPlan to handle SQL arrays
  - [x] Add proper transaction handling for multi-statement steps
  - [x] Handle step-level rollback on failure
- [x] Phase 4: Testing
  - [x] Unit tests for new PlanStep structure
  - [x] Integration tests for SQLite FK operations
  - [x] Integration tests for multi-statement execution
  - [x] Rollback tests
- [x] Phase 5: Documentation
  - [x] Update plan.json schema documentation
  - [x] Update examples with new format (via test fixtures)
  - [x] Document atomic operation patterns (in this file)

## Context

Currently, PlanStep contains a single SQL string. For operations that require multiple SQL statements (like SQLite table recreation for foreign keys), we're splitting them into multiple steps. This makes rollback harder and the plan less semantic.

**Current approach:**
```json
{
  "steps": [
    {
      "description": "Create temporary table posts_new with foreign key fk_posts_user_id",
      "sql": "CREATE TABLE posts_new (...)"
    },
    {
      "description": "Copy data from posts to posts_new",
      "sql": "INSERT INTO posts_new SELECT * FROM posts"
    },
    {
      "description": "Drop old table posts",
      "sql": "DROP TABLE posts"
    },
    {
      "description": "Rename posts_new to posts",
      "sql": "ALTER TABLE posts_new RENAME TO posts"
    }
  ]
}
```

**Proposed approach:**
```json
{
  "steps": [
    {
      "description": "Add foreign key fk_posts_user_id to table posts",
      "sql": [
        "CREATE TABLE posts_new (...)",
        "INSERT INTO posts_new SELECT * FROM posts",
        "DROP TABLE posts",
        "ALTER TABLE posts_new RENAME TO posts"
      ]
    }
  ]
}
```

## Goals

1. **Semantic clarity**: One logical operation = one step
2. **Atomic rollback**: Single rollback step reverses entire operation
3. **Transaction safety**: All SQL in a step executed in same transaction context
4. **Better testing**: Test complete atomic operations
5. **Cleaner plans**: Fewer steps, clearer intent

## Design Decisions

### 1. PlanStep Structure

**Old:**
```go
type PlanStep struct {
    Description string `json:"description"`
    SQL         string `json:"sql"`
}
```

**New:**
```go
type PlanStep struct {
    Description string   `json:"description"`
    SQL         []string `json:"sql"`  // Array of SQL statements
}
```

**Migration consideration**: Since we're not maintaining backwards compatibility, we can make a clean break. Old plans will fail to parse with new schema, which is acceptable.

### 2. Execution Strategy

Each step's SQL array will be executed sequentially within the transaction:

```go
for i, step := range plan.Steps {
    for j, sqlStmt := range step.SQL {
        trimmedSQL := strings.TrimSpace(sqlStmt)
        if trimmedSQL == "" || strings.HasPrefix(trimmedSQL, "--") {
            continue // Skip empty or comment-only statements
        }

        _, err := tx.ExecContext(ctx, sqlStmt)
        if err != nil {
            return fmt.Errorf("step %d, statement %d (%s) failed: %w",
                i, j, step.Description, err)
        }
    }
    result.StepsApplied++
}
```

### 3. Generator Method Signatures

**Current:**
```go
RecreateTableWithForeignKey(table database.Table, fk database.ForeignKey) []database.PlanStep
RecreateTableWithoutForeignKey(table database.Table, fkName string) []database.PlanStep
```

**New:**
```go
RecreateTableWithForeignKey(table database.Table, fk database.ForeignKey) database.PlanStep
RecreateTableWithoutForeignKey(table database.Table, fkName string) database.PlanStep
```

These will return a single PlanStep with multiple SQL statements in the array.

### 4. Atomic Operation Patterns

Operations that should be atomic (single step with multiple SQL):

1. **SQLite: Add foreign key** (table recreation)
   - CREATE TABLE new
   - INSERT INTO new SELECT FROM old
   - DROP TABLE old
   - ALTER TABLE new RENAME TO old

2. **SQLite: Drop foreign key** (table recreation)
   - Same pattern

3. **SQLite: Modify column** (table recreation)
   - Same pattern (for future implementation)

4. **Future: Complex multi-statement operations**
   - Adding CHECK constraints with data validation
   - Backfilling data for new NOT NULL columns
   - Splitting/merging tables

Operations that should remain separate steps:

1. **Simple DDL**: CREATE TABLE, DROP TABLE, ADD COLUMN, etc.
2. **Index operations**: CREATE INDEX, DROP INDEX
3. **Postgres FK operations**: ALTER TABLE ADD CONSTRAINT (single statement)

## Implementation Phases

### Phase 1: Design and Schema Updates

#### 1.1 Update PlanStep Structure

**File:** `internal/planner/types.go`

```go
// PlanStep represents a single logical migration operation
// that may consist of multiple SQL statements executed atomically
type PlanStep struct {
    Description string   `json:"description"`
    SQL         []string `json:"sql"` // Array of SQL statements to execute in order
}
```

#### 1.2 Update JSON Schema

**File:** `schema-json/plan.json`

Update the `sql` field definition:

```json
{
  "sql": {
    "description": "Array of SQL statements to execute for this step",
    "type": "array",
    "items": {
      "type": "string"
    },
    "minItems": 1
  }
}
```

#### 1.3 Update database.PlanStep

**File:** `database/interface.go`

```go
// PlanStep represents a single SQL operation in a migration plan
type PlanStep struct {
    Description string   `json:"description"`
    SQL         []string `json:"sql"`
}
```

### Phase 2: Core Implementation

#### 2.1 Update Generator Interface

**File:** `database/interface.go`

The interface already returns the right type for most methods. No changes needed since methods like `AddForeignKey` return `(sql string, description string)` which we'll wrap into `[]string{sql}`.

#### 2.2 Update SQLite Generator

**File:** `database/sqlite/generator.go`

Update table recreation methods:

```go
func (g *Generator) RecreateTableWithForeignKey(table database.Table, fk database.ForeignKey) database.PlanStep {
    tmpTableName := fmt.Sprintf("%s_new", table.Name)

    // Create new table with the foreign key
    newTable := table
    newTable.Name = tmpTableName
    newTable.ForeignKeys = append(newTable.ForeignKeys, fk)

    createSQL, _ := g.CreateTable(newTable)

    // Build column list
    columnNames := make([]string, len(table.Columns))
    for i, col := range table.Columns {
        columnNames[i] = col.Name
    }
    columnsStr := strings.Join(columnNames, ", ")

    // Return single step with multiple SQL statements
    return database.PlanStep{
        Description: fmt.Sprintf("Add foreign key %s to table %s", fk.Name, table.Name),
        SQL: []string{
            createSQL,
            fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s",
                tmpTableName, columnsStr, columnsStr, table.Name),
            fmt.Sprintf("DROP TABLE %s", table.Name),
            fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tmpTableName, table.Name),
        },
    }
}

func (g *Generator) RecreateTableWithoutForeignKey(table database.Table, fkName string) database.PlanStep {
    // Similar implementation
}
```

#### 2.3 Update Planner

**File:** `internal/planner/planner.go`

Update all places where we create PlanSteps to use SQL arrays:

```go
// For simple operations (single SQL statement)
sql, desc := driver.CreateTable(table)
plan.Steps = append(plan.Steps, PlanStep{
    Description: desc,
    SQL:         []string{sql}, // Wrap single statement in array
})

// For SQLite FK operations (multiple SQL statements)
if driver.Name() == "sqlite" && !driver.SupportsFeature("ALTER_ADD_FOREIGN_KEY") {
    if sqliteGen, ok := driver.(*sqlitedb.Driver); ok {
        if sourceTable != nil {
            // Returns a single PlanStep with SQL array
            step := sqliteGen.RecreateTableWithForeignKey(*sourceTable, fk)
            plan.Steps = append(plan.Steps, step)
        }
    }
} else {
    // PostgreSQL - single statement
    sql, desc := driver.AddForeignKey(tableDiff.TableName, fk)
    plan.Steps = append(plan.Steps, PlanStep{
        Description: desc,
        SQL:         []string{sql},
    })
}
```

#### 2.4 Update Plan Conversion

Update all places that convert between `database.PlanStep` and `planner.PlanStep`:

```go
// Converting database.PlanStep to planner.PlanStep
for _, dbStep := range dbSteps {
    plan.Steps = append(plan.Steps, PlanStep{
        Description: dbStep.Description,
        SQL:         dbStep.SQL,
    })
}
```

### Phase 3: Execution Engine Updates

#### 3.1 Update applyPlan

**File:** `main.go`

```go
func applyPlan(ctx context.Context, db *sql.DB, plan *planner.Plan, shadowDB *sql.DB, currentSchema *Schema, driver database.Driver) (*planner.ExecutionResult, error) {
    result := &planner.ExecutionResult{
        Success: false,
        Errors:  []string{},
    }

    // ... hash validation ...

    // If shadow DB provided, run dry-run first
    if shadowDB != nil {
        if err := dryRunPlan(ctx, shadowDB, plan, currentSchema, driver); err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("dry-run failed: %v", err))
            return result, fmt.Errorf("dry-run validation failed: %w", err)
        }
    }

    // Execute plan in a transaction
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        result.Errors = append(result.Errors, fmt.Sprintf("failed to begin transaction: %v", err))
        return result, fmt.Errorf("failed to begin transaction: %w", err)
    }

    defer func() {
        if !result.Success {
            _ = tx.Rollback()
        }
    }()

    // Execute each step
    for i, step := range plan.Steps {
        // Execute all SQL statements in this step
        for j, sqlStmt := range step.SQL {
            trimmedSQL := strings.TrimSpace(sqlStmt)
            if trimmedSQL == "" || strings.HasPrefix(trimmedSQL, "--") {
                continue // Skip empty or comment-only statements
            }

            _, err := tx.ExecContext(ctx, sqlStmt)
            if err != nil {
                errMsg := fmt.Sprintf("step %d, statement %d/%d (%s) failed: %v",
                    i+1, j+1, len(step.SQL), step.Description, err)
                result.Errors = append(result.Errors, errMsg)
                return result, fmt.Errorf("step %d failed: %w", i+1, err)
            }
        }
        result.StepsApplied++
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        result.Errors = append(result.Errors, fmt.Sprintf("failed to commit: %v", err))
        return result, fmt.Errorf("failed to commit transaction: %w", err)
    }

    result.Success = true
    return result, nil
}
```

#### 3.2 Update dryRunPlan

**File:** `main.go`

```go
func dryRunPlan(ctx context.Context, shadowDB *sql.DB, plan *planner.Plan, currentSchema *Schema, driver database.Driver) error {
    if err := applySchemaToDB(ctx, shadowDB, currentSchema, driver); err != nil {
        return fmt.Errorf("failed to prepare shadow DB: %w", err)
    }

    tx, err := shadowDB.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin shadow transaction: %w", err)
    }
    defer func() {
        _ = tx.Rollback() // Always rollback shadow DB changes
    }()

    // Execute each step
    for i, step := range plan.Steps {
        for j, sqlStmt := range step.SQL {
            trimmedSQL := strings.TrimSpace(sqlStmt)
            if trimmedSQL == "" || strings.HasPrefix(trimmedSQL, "--") {
                continue
            }

            _, err := tx.ExecContext(ctx, sqlStmt)
            if err != nil {
                return fmt.Errorf("shadow DB step %d, statement %d/%d (%s) failed: %w",
                    i+1, j+1, len(step.SQL), step.Description, err)
            }
        }
    }

    return nil
}
```

#### 3.3 Update Display Logic

**File:** `main.go` (in runApply)

```go
fmt.Fprintf(os.Stderr, "\n📋 Migration plan (%d steps):\n\n", len(plan.Steps))
for i, step := range plan.Steps {
    fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, step.Description)

    if len(step.SQL) == 1 {
        // Single statement - show inline
        sql := step.SQL[0]
        if len(sql) > 100 {
            sql = sql[:100] + "..."
        }
        fmt.Fprintf(os.Stderr, "     SQL: %s\n", sql)
    } else {
        // Multiple statements - show count and first line of each
        fmt.Fprintf(os.Stderr, "     SQL: %d statements\n", len(step.SQL))
        if *verbose {
            for j, sqlStmt := range step.SQL {
                firstLine := strings.Split(sqlStmt, "\n")[0]
                if len(firstLine) > 80 {
                    firstLine = firstLine[:80] + "..."
                }
                fmt.Fprintf(os.Stderr, "       %d. %s\n", j+1, firstLine)
            }
        }
    }
}
```

### Phase 4: Testing

#### 4.1 Update Generator Tests

**File:** `database/sqlite/generator_test.go`

Update tests to expect single PlanStep with SQL array:

```go
func TestGenerator_RecreateTableWithForeignKey(t *testing.T) {
    gen := NewGenerator()

    table := database.Table{
        Name: "posts",
        Columns: []database.Column{
            {Name: "id", Type: "integer", Nullable: false, IsPrimaryKey: true},
            {Name: "title", Type: "text", Nullable: false},
            {Name: "user_id", Type: "integer", Nullable: false},
        },
        ForeignKeys: []database.ForeignKey{},
    }

    newFK := database.ForeignKey{
        Name:              "fk_posts_user_id",
        Columns:           []string{"user_id"},
        ReferencedTable:   "users",
        ReferencedColumns: []string{"id"},
    }

    step := gen.RecreateTableWithForeignKey(table, newFK)

    // Should return single step with 4 SQL statements
    if len(step.SQL) != 4 {
        t.Fatalf("Expected 4 SQL statements, got %d", len(step.SQL))
    }

    // Statement 1: Create new table with foreign key
    if !strings.Contains(step.SQL[0], "CREATE TABLE posts_new") {
        t.Errorf("Expected statement 1 to create posts_new, got: %s", step.SQL[0])
    }
    if !strings.Contains(step.SQL[0], "CONSTRAINT fk_posts_user_id") {
        t.Errorf("Expected statement 1 to include foreign key, got: %s", step.SQL[0])
    }

    // Statement 2: Copy data
    if !strings.Contains(step.SQL[1], "INSERT INTO posts_new") {
        t.Errorf("Expected statement 2 to insert data, got: %s", step.SQL[1])
    }

    // Statement 3: Drop old table
    if step.SQL[2] != "DROP TABLE posts" {
        t.Errorf("Expected statement 3 to drop posts, got: %s", step.SQL[2])
    }

    // Statement 4: Rename new table
    if step.SQL[3] != "ALTER TABLE posts_new RENAME TO posts" {
        t.Errorf("Expected statement 4 to rename table, got: %s", step.SQL[3])
    }

    // Check description
    if !strings.Contains(step.Description, "Add foreign key fk_posts_user_id") {
        t.Errorf("Expected description about adding FK, got: %s", step.Description)
    }
}
```

#### 4.2 Integration Tests

**File:** `sqlite_integration_test.go` (new or update existing)

```go
func TestSQLite_AddForeignKey_EndToEnd(t *testing.T) {
    // Create test database
    db := getTestDB(t)
    defer db.Close()

    ctx := context.Background()

    // Create initial schema: users table
    _, err := db.ExecContext(ctx, `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            email TEXT NOT NULL
        )
    `)
    if err != nil {
        t.Fatalf("Failed to create users table: %v", err)
    }

    // Create posts table without foreign key
    _, err = db.ExecContext(ctx, `
        CREATE TABLE posts (
            id INTEGER PRIMARY KEY,
            title TEXT NOT NULL,
            user_id INTEGER NOT NULL
        )
    `)
    if err != nil {
        t.Fatalf("Failed to create posts table: %v", err)
    }

    // Insert test data
    _, err = db.ExecContext(ctx, "INSERT INTO users (id, email) VALUES (1, 'test@example.com')")
    if err != nil {
        t.Fatalf("Failed to insert user: %v", err)
    }
    _, err = db.ExecContext(ctx, "INSERT INTO posts (id, title, user_id) VALUES (1, 'Test Post', 1)")
    if err != nil {
        t.Fatalf("Failed to insert post: %v", err)
    }

    // Introspect current schema
    driver := sqlite.NewDriver()
    beforeSchema, err := driver.IntrospectSchema(ctx, db)
    if err != nil {
        t.Fatalf("Failed to introspect before schema: %v", err)
    }

    // Create desired schema with foreign key
    afterSchema := &database.Schema{
        Tables: []database.Table{
            {
                Name: "users",
                Columns: []database.Column{
                    {Name: "id", Type: "INTEGER", IsPrimaryKey: true, Nullable: false},
                    {Name: "email", Type: "TEXT", Nullable: false},
                },
            },
            {
                Name: "posts",
                Columns: []database.Column{
                    {Name: "id", Type: "INTEGER", IsPrimaryKey: true, Nullable: false},
                    {Name: "title", Type: "TEXT", Nullable: false},
                    {Name: "user_id", Type: "INTEGER", Nullable: false},
                },
                ForeignKeys: []database.ForeignKey{
                    {
                        Name:              "fk_posts_user_id",
                        Columns:           []string{"user_id"},
                        ReferencedTable:   "users",
                        ReferencedColumns: []string{"id"},
                    },
                },
            },
        },
    }

    // Generate migration plan
    diff := schema.DiffSchemas(beforeSchema, afterSchema)
    plan, err := planner.GeneratePlanWithHash(diff, beforeSchema, driver)
    if err != nil {
        t.Fatalf("Failed to generate plan: %v", err)
    }

    // Should have 1 step with 4 SQL statements
    if len(plan.Steps) != 1 {
        t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
    }
    if len(plan.Steps[0].SQL) != 4 {
        t.Fatalf("Expected step to have 4 SQL statements, got %d", len(plan.Steps[0].SQL))
    }

    // Execute plan
    result, err := applyPlanHelper(ctx, db, plan, nil, beforeSchema, driver)
    if err != nil {
        t.Fatalf("Failed to apply plan: %v", err)
    }
    if !result.Success {
        t.Fatalf("Plan execution failed: %v", result.Errors)
    }

    // Verify foreign key exists
    newSchema, err := driver.IntrospectSchema(ctx, db)
    if err != nil {
        t.Fatalf("Failed to introspect after schema: %v", err)
    }

    var postsTable *database.Table
    for i := range newSchema.Tables {
        if newSchema.Tables[i].Name == "posts" {
            postsTable = &newSchema.Tables[i]
            break
        }
    }

    if postsTable == nil {
        t.Fatal("Posts table not found after migration")
    }

    if len(postsTable.ForeignKeys) != 1 {
        t.Fatalf("Expected 1 foreign key, got %d", len(postsTable.ForeignKeys))
    }

    fk := postsTable.ForeignKeys[0]
    if fk.Name != "fk_posts_user_id" {
        t.Errorf("Expected FK name 'fk_posts_user_id', got '%s'", fk.Name)
    }

    // Verify data was preserved
    var count int
    err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts").Scan(&count)
    if err != nil {
        t.Fatalf("Failed to count posts: %v", err)
    }
    if count != 1 {
        t.Errorf("Expected 1 post after migration, got %d", count)
    }
}
```

#### 4.3 Rollback Tests

Test that rollback generation works correctly with multi-statement steps:

```go
func TestRollback_MultiStatementStep(t *testing.T) {
    // Forward plan with multi-statement step
    forwardPlan := &planner.Plan{
        Steps: []planner.PlanStep{
            {
                Description: "Add foreign key fk_posts_user_id to table posts",
                SQL: []string{
                    "CREATE TABLE posts_new (...)",
                    "INSERT INTO posts_new SELECT * FROM posts",
                    "DROP TABLE posts",
                    "ALTER TABLE posts_new RENAME TO posts",
                },
            },
        },
    }

    // Before schema (without FK)
    beforeSchema := &database.Schema{
        Tables: []database.Table{
            {
                Name: "posts",
                Columns: []database.Column{
                    {Name: "id", Type: "INTEGER", IsPrimaryKey: true},
                    {Name: "user_id", Type: "INTEGER"},
                },
                ForeignKeys: []database.ForeignKey{}, // No FK
            },
        },
    }

    driver := sqlite.NewDriver()
    rollbackPlan, err := planner.GenerateRollback(forwardPlan, beforeSchema, driver)
    if err != nil {
        t.Fatalf("Failed to generate rollback: %v", err)
    }

    // Should have 1 step that recreates the table without the FK
    if len(rollbackPlan.Steps) != 1 {
        t.Fatalf("Expected 1 rollback step, got %d", len(rollbackPlan.Steps))
    }

    // Should have multiple SQL statements
    if len(rollbackPlan.Steps[0].SQL) != 4 {
        t.Fatalf("Expected 4 SQL statements in rollback, got %d", len(rollbackPlan.Steps[0].SQL))
    }

    // Verify it's removing the FK
    if !strings.Contains(rollbackPlan.Steps[0].Description, "Drop foreign key") {
        t.Errorf("Expected rollback description about dropping FK, got: %s",
            rollbackPlan.Steps[0].Description)
    }
}
```

### Phase 5: Documentation

#### 5.1 Update Schema Documentation

**File:** `schema-json/plan.json` (add description)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Lockplane Migration Plan",
  "description": "A migration plan containing steps to migrate from one schema version to another. Each step represents a single logical operation that may consist of multiple SQL statements executed atomically.",
  "type": "object",
  "properties": {
    "source_hash": {
      "type": "string",
      "description": "SHA-256 hash of the source schema. Used to validate that the migration is being applied to the expected database state."
    },
    "steps": {
      "type": "array",
      "description": "Array of migration steps. Each step is executed atomically within a transaction.",
      "items": {
        "$ref": "#/definitions/step"
      }
    }
  },
  "required": ["steps"],
  "definitions": {
    "step": {
      "type": "object",
      "description": "A single logical migration operation. For operations that require multiple SQL statements (e.g., SQLite table recreation), all statements are included in the sql array and executed atomically.",
      "properties": {
        "description": {
          "type": "string",
          "description": "Human-readable description of what this step does"
        },
        "sql": {
          "type": "array",
          "description": "Array of SQL statements to execute for this step. All statements are executed in order within the same transaction. If any statement fails, the entire step (and transaction) is rolled back.",
          "items": {
            "type": "string"
          },
          "minItems": 1
        }
      },
      "required": ["description", "sql"]
    }
  }
}
```

#### 5.2 Update Examples

**File:** `examples/plans/add_foreign_key_sqlite.json` (new)

```json
{
  "source_hash": "abc123...",
  "steps": [
    {
      "description": "Add foreign key fk_posts_user_id to table posts",
      "sql": [
        "CREATE TABLE posts_new (id INTEGER PRIMARY KEY NOT NULL, title TEXT NOT NULL, user_id INTEGER NOT NULL, CONSTRAINT fk_posts_user_id FOREIGN KEY (user_id) REFERENCES users (id))",
        "INSERT INTO posts_new (id, title, user_id) SELECT id, title, user_id FROM posts",
        "DROP TABLE posts",
        "ALTER TABLE posts_new RENAME TO posts"
      ]
    }
  ]
}
```

**File:** `examples/plans/add_foreign_key_postgres.json` (new)

```json
{
  "source_hash": "def456...",
  "steps": [
    {
      "description": "Add foreign key fk_posts_user_id to table posts",
      "sql": [
        "ALTER TABLE posts ADD CONSTRAINT fk_posts_user_id FOREIGN KEY (user_id) REFERENCES users (id)"
      ]
    }
  ]
}
```

#### 5.3 Update README

**File:** `README.md`

Add section about atomic operations:

```markdown
## Atomic Operations

Lockplane treats each plan step as a single logical operation. For operations that require multiple SQL statements (such as adding foreign keys to existing SQLite tables), all SQL statements are included in a single step and executed atomically within a transaction.

### Example: Adding a Foreign Key in SQLite

SQLite doesn't support `ALTER TABLE ADD CONSTRAINT`, so adding a foreign key requires recreating the table. Lockplane handles this automatically as a single atomic operation:

```json
{
  "steps": [
    {
      "description": "Add foreign key fk_posts_user_id to table posts",
      "sql": [
        "CREATE TABLE posts_new (...)",
        "INSERT INTO posts_new SELECT * FROM posts",
        "DROP TABLE posts",
        "ALTER TABLE posts_new RENAME TO posts"
      ]
    }
  ]
}
```

All four SQL statements execute within a single transaction. If any statement fails, the entire operation rolls back, leaving your database unchanged.

### Rollback Safety

Because operations are atomic, rollback is simple and safe. Each forward operation has a corresponding reverse operation that's also atomic.
```

## Testing Strategy

### Unit Tests
- ✅ Generator methods return correct SQL array
- ✅ PlanStep serialization/deserialization
- ✅ Plan validation with new schema

### Integration Tests
- ✅ End-to-end FK addition on SQLite
- ✅ Data preservation during table recreation
- ✅ Transaction rollback on failure
- ✅ Shadow DB validation
- ✅ Multi-statement execution

### Regression Tests
- ✅ Existing Postgres tests still pass
- ✅ Simple operations still work
- ✅ Plan generation for all operation types

## Migration Path

Since we're not maintaining backwards compatibility:

1. **Version bump**: Increment minor version (e.g., 0.2.0 → 0.3.0)
2. **Release notes**: Clearly document breaking change in plan format
3. **Recommendation**: Regenerate any saved plans after upgrade
4. **Detection**: Old plans will fail JSON schema validation with clear error

## Risks and Mitigations

### Risk: Breaking existing saved plans
**Mitigation**: Document breaking change clearly, provide migration tool if needed

### Risk: SQL driver doesn't support multi-statement in single Exec
**Mitigation**: We execute statements individually in a loop, not concatenated

### Risk: Partial step execution in transaction
**Mitigation**: Transaction rollback on any failure ensures atomicity

### Risk: Complex rollback generation
**Mitigation**: Rollback already has access to before schema, can generate matching atomic operations

## Future Extensions

1. **Step dependencies**: Add `depends_on` field for explicit ordering
2. **Conditional steps**: Add `condition` field for environment-specific operations
3. **Step metadata**: Add `metadata` for tracking (timing, affected rows, etc.)
4. **Nested transactions**: Support savepoints for finer-grained rollback
5. **Parallel execution**: Allow marking steps as parallelizable

## References

- SQLite table recreation: https://www.sqlite.org/lang_altertable.html
- PostgreSQL transaction semantics: https://www.postgresql.org/docs/current/tutorial-transactions.html
- JSON Schema specification: https://json-schema.org/

## Timeline Estimate

- Phase 1: 2-3 hours (design, schema updates)
- Phase 2: 3-4 hours (core implementation)
- Phase 3: 2-3 hours (execution engine)
- Phase 4: 3-4 hours (testing)
- Phase 5: 1-2 hours (documentation)

**Total: 11-16 hours**

## Success Criteria

- [ ] All tests pass
- [ ] Plan JSON validates against new schema
- [ ] SQLite FK operations are single atomic steps
- [ ] Rollback works correctly for multi-statement steps
- [ ] Documentation is complete and accurate
- [ ] No regression in existing functionality
