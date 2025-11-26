# Multi-Phase Migration Plans

**Status**: 📋 Planning
**Created**: 2025-11-11
**Goal**: Automatically generate safe, multi-phase migration plans for breaking changes

---

## Progress Checklist

### Phase 1: Research & Design ✅
- [x] Review existing breaking change detection system
- [x] Design multi-phase plan structure (JSON format)
- [x] Design phase coordination mechanism
- [x] Identify which operations need multi-phase treatment
- [x] Design state tracking for migration phases
- [x] Plan integration points with existing planner
- [x] Document expand/contract patterns in detail
- [x] Implement core types: MultiPhasePlan, Phase, PhaseRollback
- [x] Implement state management: State, ActiveMigration
- [x] Write and pass state management tests
- [x] Make all design decisions

### Phase 2: Core Plan Generation ✅
- [x] Create `internal/planner/multiphase/` package
- [x] Implement phase generators for each pattern:
  - [x] Column rename (expand/contract) - `expand_contract.go`
  - [x] Column type change (dual-write) - `type_change.go`
  - [x] DROP COLUMN (deprecation period) - `deprecation.go`
  - [x] ADD NOT NULL (backfill + validation) - `validation.go`
  - [x] ADD CHECK constraint (validation phase) - `validation.go`
  - [x] DROP TABLE (deprecation period) - `drop_table.go`
- [x] Add phase validation logic - built into generators
- [x] Generate phase-specific rollback plans - each Phase has Rollback field
- [x] Add phase dependency tracking - Phase.DependsOnPhase field
- [x] Create JSON schema for multi-phase plans - `schema-json/plan-multi-phase.json`
- [x] Add MultiPhasePlan, Phase, PhaseRollback types - `internal/planner/types.go`
- [x] Write comprehensive tests - `multiphase_test.go` (98% coverage)

### Phase 3: CLI Integration ✅
- [x] Add `plan-multiphase` command - `cmd/plan_multiphase.go`
  - [x] Supports expand_contract pattern
  - [x] Supports deprecation pattern
  - [x] Supports validation pattern (NOT NULL, CHECK, UNIQUE)
  - [x] Supports type_change pattern
- [x] Add phase execution commands:
  - [x] `apply-phase` - Execute specific phase with state tracking - `cmd/apply_phase.go`
  - [x] `rollback-phase` - Rollback phase with safety checks - `cmd/rollback_phase.go`
  - [x] `phase-status` - Show current migration status - `cmd/phase_status.go`
- [x] State tracking in `.lockplane-state.json` (already implemented in Phase 1)
- [x] Phase ordering and safety checks
- [x] Interactive approval workflow (--auto-approve flag)
- [ ] Add `--multi-phase` flag to `plan` command (auto-detect from diff) - Future enhancement

### Phase 4: Phase Execution ✅
- [x] Implement phase execution engine - `cmd/apply_phase.go`
- [x] Add phase state persistence - `.lockplane-state.json` via `internal/state`
- [x] Add safety checks between phases - `state.CanExecutePhase()`
- [x] Implement rollback at any phase - `cmd/rollback_phase.go`
- [x] Add phase verification steps - Shown in apply-phase output
- [x] Handle failed phases gracefully - Added error handling with recovery instructions

### Phase 5: Testing ✅
- [x] Unit tests for phase generators - All patterns tested (expand_contract, deprecation, drop_table, validation, type_change)
- [x] State management tests - Complete test coverage for StartMigration, CompletePhase, CanExecutePhase, GetNextPhase
- [x] Test phase validation - Comprehensive validation tests for all patterns
- [x] Phase structure validation - Plan validation including phase numbers and dependencies
- [x] Rollback tests - Foreign key rollback, column rollback tests
- [x] Edge case tests - equalDefaults, LoadJSONPlan error cases

### Phase 6: Documentation ✅
- [x] Update README with multi-phase examples - Added DROP TABLE pattern, updated examples
- [x] Document all 5 patterns with examples - expand_contract, deprecation, drop_table, validation, type_change
- [x] Add execution workflow documentation - Phase-by-phase execution guide
- [x] Add error handling documentation - Recovery instructions for failed phases
- [x] Update CLI help text - All commands have comprehensive help
- [x] Add state management documentation - .lockplane-state.json tracking

### Phase 7: pgroll Integration (Future) 🔮
- [ ] Research pgroll YAML format
- [ ] Implement pgroll plan generator
- [ ] Add `--use-pgroll` flag
- [ ] Test with pgroll execution
- [ ] Document pgroll workflow

---

## Context

### What We Have (Completed)

From the **Breaking Change Detection** project:
- ✅ Safety classification system (Safe/Review/Lossy/Dangerous/Multi-Phase)
- ✅ Detectors for dangerous operations
- ✅ Text suggestions for safer alternatives
- ✅ Expand/contract pattern documentation
- ✅ Rollback safety analysis

### What We Need (This Project)

**Gap:** We detect that operations need multi-phase migrations and suggest patterns in text, but we don't **generate executable multi-phase plans**.

**Goal:** Generate actual migration plans with multiple phases that users can execute step-by-step.

---

## Design

### Multi-Phase Plan Structure

A multi-phase migration is a series of **regular migration plans** with coordination metadata:

```json
{
  "multi_phase": true,
  "operation": "rename_column",
  "description": "Rename users.email to users.email_address",
  "pattern": "expand_contract",
  "total_phases": 3,
  "current_phase": 1,
  "phases": [
    {
      "phase_number": 1,
      "name": "expand",
      "description": "Add new column and enable dual-write",
      "requires_code_deploy": true,
      "code_changes_required": [
        "Update application to write to both email and email_address columns",
        "Keep reading from email column"
      ],
      "plan": {
        "$schema": "https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/plan.json",
        "from_hash": "abc123...",
        "steps": [
          {
            "description": "Add email_address column (nullable)",
            "sql": ["ALTER TABLE users ADD COLUMN email_address TEXT"]
          },
          {
            "description": "Backfill email_address from email",
            "sql": ["UPDATE users SET email_address = email WHERE email_address IS NULL"]
          }
        ]
      },
      "verification": [
        "Verify dual-write is working: SELECT COUNT(*) FROM users WHERE email_address IS NULL",
        "Monitor application logs for email_address writes"
      ],
      "rollback": {
        "description": "Drop email_address column",
        "sql": ["ALTER TABLE users DROP COLUMN email_address"]
      }
    },
    {
      "phase_number": 2,
      "name": "migrate_reads",
      "description": "Switch application to read from new column",
      "requires_code_deploy": true,
      "depends_on_phase": 1,
      "code_changes_required": [
        "Update application to read from email_address",
        "Continue writing to both columns"
      ],
      "plan": {
        "$schema": "https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/plan.json",
        "from_hash": "def456...",
        "steps": []
      },
      "verification": [
        "Monitor application logs for email column reads (should be zero)",
        "Verify email_address is being used in queries"
      ],
      "rollback": {
        "description": "Switch reads back to email column",
        "note": "Code deployment only, no SQL changes"
      }
    },
    {
      "phase_number": 3,
      "name": "contract",
      "description": "Remove old column",
      "requires_code_deploy": true,
      "depends_on_phase": 2,
      "code_changes_required": [
        "Remove all references to email column",
        "Use only email_address column"
      ],
      "plan": {
        "$schema": "https://raw.githubusercontent.com/zakandrewking/lockplane/main/schema-json/plan.json",
        "from_hash": "ghi789...",
        "steps": [
          {
            "description": "Drop old email column",
            "sql": ["ALTER TABLE users DROP COLUMN email"]
          }
        ]
      },
      "verification": [
        "Verify application is working with new column",
        "Check for any errors related to email column"
      ],
      "rollback": {
        "description": "Re-add email column and backfill from email_address",
        "sql": [
          "ALTER TABLE users ADD COLUMN email TEXT",
          "UPDATE users SET email = email_address"
        ],
        "warning": "Rollback requires code deployment to dual-write again"
      }
    }
  ],
  "safety_notes": [
    "Each phase is backward compatible with the previous phase",
    "Code must be deployed between phases",
    "Rollback is possible at any phase but may require code changes",
    "Monitor application behavior between phases"
  ]
}
```

### Key Design Decisions

#### 1. **Phases are Regular Plans**
Each phase contains a standard `Plan` object. This means:
- Existing plan validation works
- Existing apply/rollback logic works
- Shadow DB testing works per-phase
- Can use existing JSON schema

#### 2. **State Tracking**
Multi-phase migrations need state persistence:
- `.lockplane/state.json` tracks current phase
- Prevents skipping phases
- Enables safe rollback
- Allows resuming after failure

```json
{
  "active_migration": {
    "id": "rename_users_email_20251111",
    "operation": "rename_column",
    "table": "users",
    "column": "email",
    "current_phase": 2,
    "phases_completed": [1],
    "started_at": "2025-11-11T10:00:00Z",
    "last_updated": "2025-11-11T10:30:00Z"
  }
}
```

#### 3. **Code Deployment Coordination**
Phases that require code changes:
- Have `requires_code_deploy: true`
- List specific code changes needed
- Include verification steps
- Block automatic execution

#### 4. **Pattern Templates**
Each multi-phase pattern (expand/contract, deprecation, validation) has a template:
- Generates appropriate phases
- Customizes for specific operation
- Includes standard verification steps
- Provides rollback guidance

---

## Multi-Phase Patterns

### Pattern 1: Expand/Contract (Column Rename)

**Use case:** Rename column without breaking running code

**Operations:** RENAME COLUMN, ALTER COLUMN TYPE (compatible)

**Phases:**
1. **Expand** - Add new column, enable dual-write
2. **Migrate Reads** - Switch to reading from new column
3. **Contract** - Remove old column

**Generator:**
```go
func GenerateExpandContractPlan(
    table string,
    oldColumn string,
    newColumn string,
    columnType string,
) *MultiPhasePlan
```

### Pattern 2: Deprecation Period (Column/Table Drop)

**Use case:** Safely remove column/table with deprecation period

**Operations:** DROP COLUMN, DROP TABLE

**Phases:**
1. **Mark Deprecated** - Stop writing to column/table
2. **Archive** (optional) - Export data for audit
3. **Stop Reading** - Remove all reads
4. **Drop** - Actually drop column/table

**Generator:**
```go
func GenerateDeprecationPlan(
    table string,
    column string,
    archiveData bool,
) *MultiPhasePlan
```

### Pattern 3: Validation Phase (Add Constraint)

**Use case:** Add constraint with validation to avoid locks

**Operations:** ADD NOT NULL, ADD CHECK, ADD UNIQUE

**Phases:**
1. **Backfill** - Fix existing data
2. **Add Constraint (NOT VALID)** - Add without validating old data
3. **Validate** - Validate existing rows
4. **Enforce** - Make constraint enforced

**Generator:**
```go
func GenerateValidationPhasePlan(
    table string,
    constraint string,
    backfillSQL string,
) *MultiPhasePlan
```

### Pattern 4: Type Change (Dual-Write)

**Use case:** Change column type without downtime

**Operations:** ALTER COLUMN TYPE (incompatible types)

**Phases:**
1. **Expand** - Add new column with new type
2. **Dual-Write** - Write to both columns
3. **Backfill** - Copy data to new column
4. **Migrate Reads** - Read from new column
5. **Contract** - Drop old column

**Generator:**
```go
func GenerateTypeChangePlan(
    table string,
    column string,
    oldType string,
    newType string,
    conversionExpr string,
) *MultiPhasePlan
```

---

## CLI Design

### Generate Multi-Phase Plan

```bash
# Detect operations needing multi-phase and generate plan
lockplane plan --multi-phase \
  --from current.json \
  --to desired.json > multi-phase-plan.json

# Or explicit command
lockplane plan-multi-phase \
  --operation rename_column \
  --table users \
  --from email \
  --to email_address > rename-email.json
```

### Execute Phase

```bash
# Apply phase 1 (with approval)
lockplane apply-phase multi-phase-plan.json --phase 1

# Check current phase status
lockplane phase-status

# Continue to next phase (auto-detects current phase)
lockplane apply-phase multi-phase-plan.json --next

# Skip to specific phase (dangerous, requires --force)
lockplane apply-phase multi-phase-plan.json --phase 3 --force
```

### Rollback Phase

```bash
# Rollback current phase
lockplane rollback-phase

# Rollback to specific phase
lockplane rollback-phase --to-phase 1
```

### Phase Verification

```bash
# Run verification checks for current phase
lockplane verify-phase

# Check if ready to proceed to next phase
lockplane ready-for-next-phase
```

---

## Implementation Plan

### Phase 2.1: Core Types

Add to `internal/planner/types.go`:

```go
// MultiPhasePlan represents a migration requiring multiple coordinated steps
type MultiPhasePlan struct {
    MultiPhase          bool              `json:"multi_phase"`
    Operation           string            `json:"operation"`
    Description         string            `json:"description"`
    Pattern             string            `json:"pattern"` // expand_contract, deprecation, validation, dual_write
    TotalPhases         int               `json:"total_phases"`
    CurrentPhase        int               `json:"current_phase"`
    Phases              []Phase           `json:"phases"`
    SafetyNotes         []string          `json:"safety_notes"`
}

// Phase represents a single phase in a multi-phase migration
type Phase struct {
    PhaseNumber         int               `json:"phase_number"`
    Name                string            `json:"name"`
    Description         string            `json:"description"`
    RequiresCodeDeploy  bool              `json:"requires_code_deploy"`
    DependsOnPhase      int               `json:"depends_on_phase,omitempty"`
    CodeChangesRequired []string          `json:"code_changes_required,omitempty"`
    Plan                *Plan             `json:"plan"`
    Verification        []string          `json:"verification"`
    Rollback            *PhaseRollback    `json:"rollback"`
}

// PhaseRollback describes how to rollback a phase
type PhaseRollback struct {
    Description string   `json:"description"`
    SQL         []string `json:"sql,omitempty"`
    Note        string   `json:"note,omitempty"`
    Warning     string   `json:"warning,omitempty"`
}
```

### Phase 2.2: Pattern Generators

Create `internal/planner/multiphase/` package:

```
internal/planner/multiphase/
├── expand_contract.go  // Column rename, compatible type changes
├── deprecation.go      // DROP COLUMN, DROP TABLE
├── validation.go       // ADD NOT NULL, ADD CHECK
├── type_change.go      // Incompatible type changes
└── generator.go        // Main multi-phase plan generator
```

### Phase 2.3: Integration with Planner

Update `internal/planner/planner.go`:

```go
// CheckNeedsMultiPhase determines if a diff requires multi-phase migration
func CheckNeedsMultiPhase(diff *schema.Diff) (bool, string) {
    // Check safety classification
    // Return true + pattern name if multi-phase needed
}

// GenerateMultiPhasePlan creates a multi-phase plan for a breaking change
func GenerateMultiPhasePlan(
    diff *schema.Diff,
    pattern string,
    fromSchema *schema.Schema,
    toSchema *schema.Schema,
    driver database.Driver,
) (*MultiPhasePlan, error) {
    // Route to appropriate pattern generator
}
```

### Phase 3: State Management

Create `internal/state/` package:

```go
// State tracks multi-phase migration progress
type State struct {
    ActiveMigration *ActiveMigration `json:"active_migration,omitempty"`
}

// ActiveMigration tracks the currently running multi-phase migration
type ActiveMigration struct {
    ID               string    `json:"id"`
    Operation        string    `json:"operation"`
    Table            string    `json:"table"`
    CurrentPhase     int       `json:"current_phase"`
    PhasesCompleted  []int     `json:"phases_completed"`
    StartedAt        time.Time `json:"started_at"`
    LastUpdated      time.Time `json:"last_updated"`
}

// Load reads state from .lockplane/state.json
func Load() (*State, error)

// Save writes state to .lockplane/state.json
func (s *State) Save() error

// CanProceedToPhase checks if safe to execute a phase
func (s *State) CanProceedToPhase(phase int) error
```

---

## Example Workflow

### User Wants to Rename Column

```bash
# 1. User introspects current state
lockplane introspect --target postgres://localhost/db > current.json

# 2. User edits desired schema (rename email → email_address)
vim desired.json

# 3. User generates plan - Lockplane detects rename needs multi-phase
lockplane plan --from current.json --to desired.json --validate

# Output:
# ⚠️  This migration requires multi-phase deployment
# Operation: RENAME COLUMN users.email → users.email_address
# Pattern: Expand/Contract (3 phases)
#
# Generate multi-phase plan with:
#   lockplane plan --multi-phase --from current.json --to desired.json

# 4. User generates multi-phase plan
lockplane plan --multi-phase \
  --from current.json \
  --to desired.json > rename-email-plan.json

# 5. Review generated plan
cat rename-email-plan.json
# Shows 3 phases with descriptions, SQL, code changes needed

# 6. Execute Phase 1
lockplane apply-phase rename-email-plan.json --phase 1
# Output:
# Phase 1/3: Expand
# ✓ Added email_address column
# ✓ Backfilled data
#
# Next steps:
#   1. Deploy code changes (see phase 1 code_changes_required)
#   2. Verify dual-write is working
#   3. Run: lockplane apply-phase rename-email-plan.json --phase 2

# 7. Deploy code changes (dual-write to both columns)
git push && deploy

# 8. Verify phase 1
lockplane verify-phase
# Runs verification queries, checks dual-write

# 9. Execute Phase 2
lockplane apply-phase rename-email-plan.json --phase 2
# (Migrate reads to new column - code deployment only)

# 10. Deploy code changes (read from email_address)
git push && deploy

# 11. Execute Phase 3
lockplane apply-phase rename-email-plan.json --phase 3
# ✓ Dropped old email column
#
# Multi-phase migration complete!
```

---

## Testing Strategy

### Unit Tests

Each pattern generator:
- Test phase generation
- Test code change suggestions
- Test verification steps
- Test rollback generation

### Integration Tests

End-to-end workflows:
- Generate multi-phase plan
- Execute each phase
- Verify state tracking
- Test rollback at each phase
- Test failure recovery

### Manual Testing

Real-world scenarios:
- Column rename on test database
- Type change with production-like data
- DROP COLUMN with deprecation
- Constraint addition with backfill

---

## Success Criteria

1. ✅ Users can generate multi-phase plans automatically
2. ✅ Each phase is backward compatible
3. ✅ Code changes are clearly documented
4. ✅ Verification steps prevent errors
5. ✅ Rollback works at any phase
6. ✅ State tracking prevents skipping phases
7. ✅ Works with existing shadow DB validation
8. ✅ Clear documentation with examples

---

## Future Enhancements

### pgroll Integration

Generate pgroll YAML instead of multi-phase JSON:

```bash
lockplane plan --use-pgroll \
  --from current.json \
  --to desired.json > pgroll-migration.yaml
```

### gh-ost Integration (MySQL)

Generate gh-ost command for MySQL:

```bash
lockplane plan --use-gh-ost \
  --from current.json \
  --to desired.json > gh-ost-command.sh
```

### Automated Phase Execution

With confidence, auto-execute phases:

```bash
lockplane apply-multi-phase rename-email-plan.json \
  --auto \
  --pause-between-phases 1h \
  --verify-each-phase
```

### Phase Monitoring

Track metrics between phases:

```bash
lockplane monitor-phase --metrics \
  --alert-on-errors \
  --slack-webhook https://...
```

---

## Design Decisions

### 1. State Storage Location ✅

**Decision**: Use `.lockplane-state.json` in project root (git-ignored)

**Rationale**:
- Simple file-based storage, no database dependency
- Easy to inspect and debug
- Git-ignored to avoid conflicts
- Atomic writes (temp file + rename)
- Version-controlled structure
- Can be backed up/restored easily

**Alternative considered**: Database table `lockplane.migration_state`
- Rejected: Adds complexity, requires database access for state queries
- May revisit for multi-user scenarios

**Implementation**: `internal/state/state.go`

### 2. Concurrent Migrations ✅

**Decision**: Block concurrent multi-phase migrations

**Rationale**:
- Multi-phase migrations require coordination and state tracking
- Concurrent migrations on same table could conflict
- Clear error message: "another migration is already in progress"
- Simple to implement and reason about
- Users can explicitly cancel/complete before starting new migration

**Future enhancement**: Allow concurrent migrations on different tables with dependency checking

**Implementation**: `State.StartMigration()` checks for active migration

### 3. Phase Transitions ✅

**Decision**: Manual approval required between phases

**Rationale**:
- Safety first: each phase requires explicit action
- Allows time for code deployment and verification
- User can inspect state between phases
- Prevents accidental progression
- CLI shows clear next steps

**Command pattern**:
```bash
# Execute specific phase
lockplane apply-phase plan.json --phase 2

# Execute next phase (auto-detects from state)
lockplane apply-phase plan.json --next
```

**Future enhancement**: `--auto` flag for automated execution with verification checks

**Implementation**: Phase execution checks `State.CanExecutePhase()`

### 4. CI/CD Integration 🔮

**Decision**: Manual workflow first, CI/CD templates later

**Rationale**:
- Phase 1 focuses on core functionality
- CI/CD patterns emerge from real usage
- Different teams have different workflows
- Can provide examples in documentation

**Future work**:
- GitHub Actions workflow templates
- Phase execution artifacts
- Automated verification scripts
- Slack/email notifications between phases

---

## References

- Completed: `devdocs/projects/completed/breaking_change_detection.md`
- Roadmap: `devdocs/roadmap.md` (Priority #2)
- pgroll: https://github.com/xataio/pgroll
- Evolutionary Database Design: https://martinfowler.com/articles/evodb.html
- Expand/Contract Pattern: https://www.tim-wellhausen.de/papers/ExpandAndContract.pdf
