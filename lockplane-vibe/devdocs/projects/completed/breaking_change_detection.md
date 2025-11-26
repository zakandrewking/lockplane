# Breaking Change Detection

**Status**: ✅ Completed
**Created**: 2025-11-09
**Completed**: 2025-11-11
**Goal**: Automatically detect dangerous migrations and classify them by safety/rollback risk

---

## Progress Checklist

### Phase 1: Design and Analysis
- [x] Create design document
- [x] Review existing validation system
- [x] Design classification taxonomy (Safe/Review/Lossy/Dangerous/Multi-Phase)
- [x] Define breaking change catalog
- [x] Plan safer alternative suggestions

### Phase 2: Core Implementation
- [x] Implement SafetyClassification type and methods
- [x] Extend ValidationResult with safety classification
- [x] Implement breaking change detectors:
  - [x] DROP COLUMN detector
  - [x] DROP TABLE detector
  - [x] ALTER COLUMN TYPE detector
  - [x] ADD NOT NULL detector (enhanced AddColumnValidator)
  - [ ] DROP CONSTRAINT detector (deferred)
  - [ ] MODIFY COLUMN (nullable → NOT NULL) detector (deferred)
- [x] Add rollback safety analysis

### Phase 3: Safer Alternatives
- [x] Implement alternative suggestion system
- [x] Add multi-phase migration guidance
- [x] Add expand/contract pattern suggestions
- [x] Document zero-downtime patterns (in design doc)

### Phase 4: Testing
- [x] Unit tests for each detector (16 new tests)
- [x] Integration tests for classification
- [x] Test safer alternative suggestions
- [x] Test with real-world migration scenarios

### Phase 5: Documentation & Polish
- [x] Update CLI output to show safety classification
- [x] Add documentation for safety levels (README)
- [x] Update README with breaking change examples
- [x] Update llms.txt with safety features
- [x] Commit and push all changes

---

## Context

### Current State

Lockplane has a basic validation system in `validation.go`:
- `ValidationResult` with `Valid`, `Reversible`, `Errors`, `Warnings`, `Reasons`
- `AddColumnValidator` - checks NOT NULL without DEFAULT
- `AddForeignKeyValidator` - validates FK references
- `ValidateSchemaDiff()` - validates entire diffs

**Gaps:**
1. No classification of migration safety (Safe/Lossy/Dangerous)
2. No detection of breaking changes (DROP COLUMN, ALTER TYPE, etc.)
3. No guidance on safer alternatives
4. No multi-phase migration suggestions
5. Rollback safety not analyzed beyond boolean flag

### Production Pain Points

Teams struggle with:
- **Accidental data loss**: DROP COLUMN destroys data permanently
- **Breaking deployments**: ALTER COLUMN TYPE breaks running apps
- **Lock contention**: Heavy DDL operations block production traffic
- **Unclear rollback paths**: Don't know if rollback is safe or lossy
- **No warning about risks**: Dangerous operations execute without review

**Example scenarios:**
- Developer drops column with active data, can't recover
- ALTER TYPE fails at runtime, migration stuck halfway
- ADD NOT NULL fails on existing rows, requires manual intervention
- Rollback loses data written during migration, users complain

---

## Goals

1. **Automatic Detection**: Identify breaking changes without manual review
2. **Safety Classification**: Categorize migrations by risk level
3. **Rollback Analysis**: Predict data loss from rollbacks
4. **Safer Alternatives**: Suggest multi-phase or zero-downtime approaches
5. **Production Safety**: Prevent dangerous operations from reaching prod

---

## Design

### Safety Classification Taxonomy

Every migration operation gets classified into one of these categories:

#### 1. **Safe** ✅
Operations that:
- Can be applied without breaking running applications
- Are fully reversible without data loss
- Don't require multi-phase deployment
- Have minimal lock contention

**Examples:**
- ADD COLUMN (nullable)
- ADD COLUMN (NOT NULL with DEFAULT)
- CREATE INDEX CONCURRENTLY (Postgres)
- ADD FOREIGN KEY (with valid data)
- CREATE TABLE

**Characteristics:**
- Backward compatible
- Rollback loses no data
- Safe to apply to production
- Old code continues working

#### 2. **Requires Review** ⚠️
Operations that:
- Might break running applications
- Are reversible but may lose data
- Should be reviewed before production
- May require coordination

**Examples:**
- DROP INDEX (if queries rely on it)
- DROP FOREIGN KEY (if app assumes it exists)
- ADD FOREIGN KEY (might fail if data invalid)
- ALTER COLUMN DEFAULT (changes behavior for new rows)
- MODIFY COLUMN (nullable → NOT NULL with DEFAULT)

**Characteristics:**
- Potentially breaking
- Rollback might lose data
- Needs manual review
- May need testing

#### 3. **Lossy** 🔶
Operations where:
- Rollback will lose data written in new schema
- Application must coordinate with schema change
- Multi-phase deployment recommended
- Backward compatibility breaks

**Examples:**
- ADD COLUMN NOT NULL (requires backfill, rollback loses data)
- ALTER COLUMN TYPE (data might not fit old type after rollback)
- MODIFY COLUMN (NOT NULL → nullable, rollback fails if nulls written)
- SPLIT TABLE (rollback complex)

**Characteristics:**
- Rollback loses some data
- Requires application deployment
- Multi-phase migration recommended
- Test thoroughly on shadow DB

#### 4. **Dangerous** ❌
Operations that:
- Permanently destroy data
- Cannot be rolled back safely
- Break running applications immediately
- Require multi-phase deployment

**Examples:**
- DROP COLUMN
- DROP TABLE
- ALTER COLUMN TYPE (without backfill/dual-write)
- DROP NOT NULL (without validation phase)

**Characteristics:**
- Data loss is permanent
- Rollback impossible or very lossy
- Requires multi-phase deployment
- Should use expand/contract pattern

#### 5. **Multi-Phase Required** 🔄
Operations that:
- Must be split into multiple steps
- Require code deployment between steps
- Use expand/contract pattern
- Need careful coordination

**Examples:**
- DROP COLUMN → deprecate, remove reads, remove column
- RENAME COLUMN → dual-write, migrate reads, remove old
- ALTER COLUMN TYPE → add new, backfill, dual-write, migrate reads, drop old
- SPLIT TABLE → add new table, dual-write, backfill, migrate reads, drop old

**Characteristics:**
- Cannot be done in single migration
- Requires coordination with code
- 3-5 phase deployment
- Each phase is backward compatible

---

### Breaking Change Catalog

Comprehensive list of operations and their classifications:

#### Table Operations

| Operation | Safety | Reversible? | Rollback Impact | Requires Multi-Phase? |
|-----------|--------|-------------|-----------------|----------------------|
| CREATE TABLE | ✅ Safe | Yes | No data loss | No |
| DROP TABLE | ❌ Dangerous | No | All table data lost | Yes (deprecation) |
| RENAME TABLE | 🔄 Multi-Phase | Yes but breaks apps | No data loss | Yes |

#### Column Operations

| Operation | Safety | Reversible? | Rollback Impact | Requires Multi-Phase? |
|-----------|--------|-------------|-----------------|----------------------|
| ADD COLUMN (nullable) | ✅ Safe | Yes | No data loss | No |
| ADD COLUMN (NOT NULL + DEFAULT) | ✅ Safe | Yes | No data loss | No |
| ADD COLUMN (NOT NULL, no DEFAULT) | ❌ Dangerous | No | Fails on existing rows | Yes |
| DROP COLUMN | ❌ Dangerous | No | Column data lost permanently | Yes (deprecation) |
| RENAME COLUMN | 🔄 Multi-Phase | Yes but breaks apps | No data loss | Yes |
| ALTER COLUMN TYPE | 🔶 Lossy | Depends | Data might not fit old type | Yes (dual-write) |
| ALTER COLUMN (nullable → NOT NULL) | 🔶 Lossy | Yes | Fails if nulls written | Yes (backfill + validation) |
| ALTER COLUMN (NOT NULL → nullable) | ⚠️ Review | Yes | May lose validation | No |
| ALTER COLUMN DEFAULT | ⚠️ Review | Yes | Changes default behavior | No |
| DROP COLUMN DEFAULT | ⚠️ Review | Yes | Changes default behavior | No |

#### Constraint Operations

| Operation | Safety | Reversible? | Rollback Impact | Requires Multi-Phase? |
|-----------|--------|-------------|-----------------|----------------------|
| ADD FOREIGN KEY (valid data) | ✅ Safe | Yes | No data loss | No |
| ADD FOREIGN KEY (might fail) | ⚠️ Review | Yes | May fail on invalid data | No |
| DROP FOREIGN KEY | ⚠️ Review | Yes | Loses referential integrity | No |
| ADD PRIMARY KEY | 🔄 Multi-Phase | Depends | May fail, requires validation | Yes |
| DROP PRIMARY KEY | ❌ Dangerous | No | Loses uniqueness guarantee | Yes |
| ADD UNIQUE CONSTRAINT | 🔶 Lossy | Depends | May fail on duplicates | Yes (validate first) |
| DROP UNIQUE CONSTRAINT | ⚠️ Review | Yes | Allows duplicates | No |
| ADD CHECK CONSTRAINT | 🔶 Lossy | Depends | May fail on existing data | Yes (validate first) |
| DROP CHECK CONSTRAINT | ⚠️ Review | Yes | Allows invalid data | No |

#### Index Operations

| Operation | Safety | Reversible? | Rollback Impact | Requires Multi-Phase? |
|-----------|--------|-------------|-----------------|----------------------|
| CREATE INDEX | ⚠️ Review | Yes | May hold lock | No |
| CREATE INDEX CONCURRENTLY | ✅ Safe | Yes | No lock | No |
| DROP INDEX | ⚠️ Review | Yes | Query performance impact | No |
| RENAME INDEX | ✅ Safe | Yes | No data impact | No |

---

### Rollback Safety Analysis

For each operation, analyze rollback impact:

#### Safe Rollback (No Data Loss)
- ADD COLUMN (nullable) → DROP COLUMN
- CREATE INDEX → DROP INDEX
- ADD TABLE → DROP TABLE (if table empty)

#### Lossy Rollback (Some Data Loss)
- ADD COLUMN (NOT NULL + DEFAULT) → DROP COLUMN (loses data written to column)
- ALTER COLUMN TYPE → ALTER COLUMN TYPE back (data might not fit)
- ALTER COLUMN (nullable → NOT NULL) → ALTER back (fails if nulls written)

#### Impossible Rollback (Permanent Data Loss)
- DROP COLUMN → Cannot recreate with original data
- DROP TABLE → Cannot recreate with original data
- ALTER COLUMN TYPE (destructive) → Data already transformed/lost

#### Rollback Analysis Output

```
Migration Safety Report

Operation: DROP COLUMN users.last_login
Safety: ❌ Dangerous
Reversible: No - Permanent data loss

Rollback Analysis:
  ✗ Cannot rollback this operation
  ✗ Column data will be permanently lost
  ✗ Estimated impact: ~1.2M rows × 8 bytes = 9.6 MB data loss

Safer Alternative:
  Use multi-phase deprecation pattern:
    Phase 1: Stop writing to column (code deploy)
    Phase 2: Archive data elsewhere if needed
    Phase 3: Drop column (safe - no active writes)

  Or use expand/contract:
    Phase 1: Add new column, dual-write (code + schema)
    Phase 2: Backfill new column from old
    Phase 3: Migrate reads to new column (code deploy)
    Phase 4: Drop old column (safe - no active use)
```

---

### Safer Alternative Suggestions

For each dangerous operation, suggest safer approaches:

#### Pattern 1: Expand/Contract (Column Rename/Type Change)

**Problem:** Want to rename `email` → `email_address`

**Naive approach (Dangerous):**
```sql
ALTER TABLE users RENAME COLUMN email TO email_address;
-- ❌ Breaks all running code immediately
```

**Safer approach (Multi-Phase):**
```
Phase 1: Expand
  - ADD COLUMN email_address TEXT
  - Deploy code: write to both columns
  - Backfill: UPDATE users SET email_address = email WHERE email_address IS NULL

Phase 2: Migrate Reads
  - Deploy code: read from email_address, write to both
  - Monitor: ensure all services using new column

Phase 3: Contract
  - Deploy code: only use email_address
  - DROP COLUMN email (safe - no active use)
```

#### Pattern 2: Deprecation Period (Column/Table Drop)

**Problem:** Want to drop unused `is_verified` column

**Naive approach (Dangerous):**
```sql
DROP COLUMN is_verified;
-- ❌ If column still used, breaks app immediately
-- ❌ Data permanently lost
```

**Safer approach (Multi-Phase):**
```
Phase 1: Mark Deprecated
  - Add @deprecated annotation in schema
  - Deploy code: remove all writes to column
  - Monitor: verify no active writes

Phase 2: Archive (Optional)
  - Export column data if needed for audit/recovery
  - Store in backup table or data warehouse

Phase 3: Remove Reads
  - Deploy code: remove all reads from column
  - Monitor: verify no active reads

Phase 4: Drop Column
  - DROP COLUMN is_verified (safe - no active use)
```

#### Pattern 3: Validation Phase (Add Constraint)

**Problem:** Want to add NOT NULL to existing column

**Naive approach (Dangerous):**
```sql
ALTER COLUMN email SET NOT NULL;
-- ❌ Fails if any NULL values exist
```

**Safer approach (Multi-Phase):**
```
Phase 1: Backfill
  - UPDATE users SET email = 'placeholder@example.com' WHERE email IS NULL
  - Or reject: DELETE FROM users WHERE email IS NULL

Phase 2: Add Constraint (NOT VALID)
  - ALTER TABLE users ADD CONSTRAINT users_email_not_null
    CHECK (email IS NOT NULL) NOT VALID
  - New rows validated, old rows ignored

Phase 3: Validate Constraint
  - ALTER TABLE users VALIDATE CONSTRAINT users_email_not_null
  - Validates existing rows with ShareUpdateExclusive lock (lighter than full table lock)

Phase 4: Make NOT NULL
  - ALTER COLUMN email SET NOT NULL
  - Safe now - constraint already validated
```

#### Pattern 4: Zero-Downtime Tools

**Problem:** Need to change column type on large table

**Naive approach (High Lock Contention):**
```sql
ALTER TABLE logs ALTER COLUMN created_at TYPE TIMESTAMPTZ;
-- ❌ Holds AccessExclusive lock for minutes
-- ❌ Blocks all reads and writes
```

**Safer approach (Use Specialized Tool):**
```
For PostgreSQL: Use pgroll
  - Creates shadow table with new schema
  - Triggers keep tables in sync
  - Switch atomically with minimal lock
  - Lockplane can generate pgroll config

For MySQL: Use gh-ost or pt-online-schema-change
  - Similar shadow table approach
  - Lockplane can generate gh-ost commands
```

---

## Implementation Plan

### Data Structures

```go
// SafetyLevel represents how safe a migration operation is
type SafetyLevel int

const (
	SafetyLevelSafe       SafetyLevel = iota // ✅ Safe to apply, fully reversible
	SafetyLevelReview                        // ⚠️ Needs review, might be risky
	SafetyLevelLossy                         // 🔶 Lossy rollback, requires care
	SafetyLevelDangerous                     // ❌ Dangerous, permanent data loss
	SafetyLevelMultiPhase                    // 🔄 Requires multi-phase deployment
)

func (s SafetyLevel) String() string {
	switch s {
	case SafetyLevelSafe:
		return "Safe"
	case SafetyLevelReview:
		return "Requires Review"
	case SafetyLevelLossy:
		return "Lossy"
	case SafetyLevelDangerous:
		return "Dangerous"
	case SafetyLevelMultiPhase:
		return "Multi-Phase Required"
	default:
		return "Unknown"
	}
}

func (s SafetyLevel) Icon() string {
	switch s {
	case SafetyLevelSafe:
		return "✅"
	case SafetyLevelReview:
		return "⚠️"
	case SafetyLevelLossy:
		return "🔶"
	case SafetyLevelDangerous:
		return "❌"
	case SafetyLevelMultiPhase:
		return "🔄"
	default:
		return "❓"
	}
}

// SafetyClassification contains safety analysis for a migration
type SafetyClassification struct {
	Level                SafetyLevel
	BreakingChange       bool     // Does this break running apps?
	DataLoss             bool     // Does this cause permanent data loss?
	RollbackDataLoss     bool     // Does rollback lose data?
	RequiresMultiPhase   bool     // Must be split into multiple migrations?
	LockContention       bool     // Will this hold heavyweight locks?
	RollbackDescription  string   // What happens on rollback?
	SaferAlternatives    []string // Suggested safer approaches
}

// Extend ValidationResult with safety analysis
type ValidationResult struct {
	Valid         bool
	Reversible    bool
	Errors        []string
	Warnings      []string
	Reasons       []string

	// New: Safety classification
	Safety        *SafetyClassification `json:"safety,omitempty"`
}
```

### Detector Interface

```go
// BreakingChangeDetector identifies dangerous operations
type BreakingChangeDetector interface {
	// Detect analyzes a diff and returns safety classifications
	Detect(diff *schema.SchemaDiff) []SafetyClassification
}

// Implement detectors for each operation type
type DropColumnDetector struct{}
type DropTableDetector struct{}
type AlterColumnTypeDetector struct{}
type AddNotNullDetector struct{}
type DropConstraintDetector struct{}
type AlterNullabilityDetector struct{}
```

### Validator Extensions

Update existing validators to include safety analysis:

```go
// Enhanced AddColumnValidator
func (v *AddColumnValidator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:      true,
		Reversible: true,
		Errors:     []string{},
		Warnings:   []string{},
		Reasons:    []string{},
	}

	// Existing logic...

	// New: Safety classification
	if !v.Column.Nullable && (v.Column.Default == nil || *v.Column.Default == "") {
		// NOT NULL without DEFAULT - dangerous
		result.Safety = &SafetyClassification{
			Level:              SafetyLevelDangerous,
			BreakingChange:     true,
			DataLoss:           false,
			RollbackDataLoss:   false,
			RequiresMultiPhase: true,
			LockContention:     false,
			RollbackDescription: "Rollback will drop column, losing any data written to it",
			SaferAlternatives: []string{
				"Add column as nullable first",
				"Add column with DEFAULT value",
				"Use multi-phase: add nullable, backfill, make NOT NULL",
			},
		}
	} else if v.Column.Nullable {
		// Nullable column - safe
		result.Safety = &SafetyClassification{
			Level:               SafetyLevelSafe,
			BreakingChange:      false,
			DataLoss:            false,
			RollbackDataLoss:    true, // Rollback drops column with any written data
			RequiresMultiPhase:  false,
			LockContention:      false,
			RollbackDescription: "Rollback will drop column. Data written to this column will be lost.",
		}
	} else if v.Column.Default != nil {
		// NOT NULL with DEFAULT - safe
		result.Safety = &SafetyClassification{
			Level:               SafetyLevelSafe,
			BreakingChange:      false,
			DataLoss:            false,
			RollbackDataLoss:    true,
			RequiresMultiPhase:  false,
			LockContention:      false,
			RollbackDescription: "Rollback will drop column. Data written to this column will be lost.",
		}
	}

	return result
}
```

### New Validators

```go
// DropColumnValidator - NEW
type DropColumnValidator struct {
	TableName  string
	Column     Column
	RowCount   int64 // Optional: from shadow DB analysis
	ColumnSize int64 // Optional: estimated data loss
}

func (v *DropColumnValidator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:      true, // Valid but dangerous
		Reversible: false,
		Errors:     []string{},
		Warnings: []string{
			fmt.Sprintf("⚠️ Dropping column '%s.%s' will permanently lose data",
				v.TableName, v.Column.Name),
		},
		Reasons: []string{
			"DROP COLUMN is irreversible - data cannot be recovered",
		},
		Safety: &SafetyClassification{
			Level:              SafetyLevelDangerous,
			BreakingChange:     true,
			DataLoss:           true,
			RollbackDataLoss:   false, // Can't rollback
			RequiresMultiPhase: true,
			LockContention:     true, // Holds AccessExclusive lock
			RollbackDescription: "Cannot rollback - column data is permanently lost",
			SaferAlternatives: []string{
				"Use deprecation period: stop writes → archive data → stop reads → drop column",
				"Use expand/contract if renaming: add new column → dual-write → migrate reads → drop old",
			},
		},
	}

	if v.RowCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Estimated data loss: %d rows", v.RowCount))
	}

	if v.ColumnSize > 0 {
		sizeMB := float64(v.ColumnSize) / (1024 * 1024)
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Estimated data loss: %.2f MB", sizeMB))
	}

	return result
}

// AlterColumnTypeValidator - NEW
type AlterColumnTypeValidator struct {
	TableName string
	ColumnName string
	OldType   string
	NewType   string
}

func (v *AlterColumnTypeValidator) Validate() ValidationResult {
	// Analyze type conversion safety
	conversionSafe := isTypConversionSafe(v.OldType, v.NewType)
	rollbackSafe := isTypeConversionSafe(v.NewType, v.OldType)

	var level SafetyLevel
	var alternatives []string

	if !conversionSafe {
		level = SafetyLevelDangerous
		alternatives = []string{
			"Use multi-phase: add new column → backfill → dual-write → migrate reads → drop old",
			"Test conversion on shadow DB first",
			"Consider using a USING expression to handle conversion",
		}
	} else if !rollbackSafe {
		level = SafetyLevelLossy
		alternatives = []string{
			"Test rollback on shadow DB to verify data fits",
			"Consider if this change is truly necessary",
		}
	} else {
		level = SafetyLevelReview
	}

	return ValidationResult{
		Valid:      conversionSafe,
		Reversible: rollbackSafe,
		Warnings: []string{
			fmt.Sprintf("Changing column type: %s → %s", v.OldType, v.NewType),
		},
		Safety: &SafetyClassification{
			Level:              level,
			BreakingChange:     true,
			DataLoss:           !conversionSafe,
			RollbackDataLoss:   !rollbackSafe,
			RequiresMultiPhase: !conversionSafe,
			LockContention:     true,
			RollbackDescription: fmt.Sprintf(
				"Rollback will convert %s → %s. Data might not fit old type.",
				v.NewType, v.OldType,
			),
			SaferAlternatives: alternatives,
		},
	}
}

// Helper: Check if type conversion is safe
func isTypeConversionSafe(from, to string) bool {
	// Widening conversions (safe)
	safeConversions := map[string][]string{
		"SMALLINT": {"INTEGER", "BIGINT", "NUMERIC"},
		"INTEGER":  {"BIGINT", "NUMERIC"},
		"BIGINT":   {"NUMERIC"},
		"REAL":     {"DOUBLE PRECISION", "NUMERIC"},
		"DOUBLE PRECISION": {"NUMERIC"},
		"VARCHAR":  {"TEXT"},
		"CHAR":     {"VARCHAR", "TEXT"},
		"DATE":     {"TIMESTAMP", "TIMESTAMPTZ"},
		"TIMESTAMP": {"TIMESTAMPTZ"},
	}

	// Normalize types
	from = normalizeType(from)
	to = normalizeType(to)

	if safe, ok := safeConversions[from]; ok {
		for _, safeType := range safe {
			if safeType == to {
				return true
			}
		}
	}

	return false
}
```

---

## CLI Output Updates

### Current Output
```
✓ Migration plan generated successfully
  Forward: 3 steps
  Rollback: 3 steps
```

### Enhanced Output with Safety Classification
```
Migration Safety Report

Operation: Drop column users.last_login
  Safety: ❌ Dangerous - Permanent data loss
  Breaking: Yes - Will break running applications
  Reversible: No

  ⚠️ Warnings:
    • Column data will be permanently lost
    • Cannot be rolled back
    • Will hold AccessExclusive lock

  💡 Safer Alternatives:
    1. Use deprecation period:
       - Phase 1: Stop writing to column (code deploy)
       - Phase 2: Stop reading from column (code deploy)
       - Phase 3: Drop column (safe - no active use)

    2. Archive data first:
       - Export column data to backup table
       - Then drop column

Operation: Add column users.email_verified (NOT NULL, DEFAULT false)
  Safety: ✅ Safe
  Breaking: No
  Reversible: Yes (but rollback loses new data)

  ℹ️ Rollback Impact:
    • Rollback will drop column
    • Any data written to email_verified will be lost

Summary:
  ❌ 1 dangerous operation
  ✅ 1 safe operation

  ⚠️ Review required before applying to production
```

---

## Testing Strategy

### Unit Tests

```go
func TestDropColumnValidator_Dangerous(t *testing.T) {
	validator := &DropColumnValidator{
		TableName: "users",
		Column: Column{Name: "last_login", Type: "TIMESTAMP"},
	}

	result := validator.Validate()

	if result.Safety.Level != SafetyLevelDangerous {
		t.Errorf("Expected Dangerous, got %v", result.Safety.Level)
	}

	if !result.Safety.DataLoss {
		t.Error("Expected data loss flag")
	}

	if len(result.Safety.SaferAlternatives) == 0 {
		t.Error("Expected safer alternatives")
	}
}

func TestAlterColumnType_SafeWidening(t *testing.T) {
	validator := &AlterColumnTypeValidator{
		TableName:  "users",
		ColumnName: "age",
		OldType:    "INTEGER",
		NewType:    "BIGINT",
	}

	result := validator.Validate()

	if result.Safety.Level == SafetyLevelDangerous {
		t.Error("Widening conversion should not be dangerous")
	}

	if !result.Valid {
		t.Error("Widening conversion should be valid")
	}
}

func TestAlterColumnType_DangerousNarrowing(t *testing.T) {
	validator := &AlterColumnTypeValidator{
		TableName:  "users",
		ColumnName: "score",
		OldType:    "BIGINT",
		NewType:    "INTEGER",
	}

	result := validator.Validate()

	if result.Safety.Level != SafetyLevelDangerous {
		t.Error("Narrowing conversion should be dangerous")
	}

	if result.Valid {
		t.Error("Narrowing conversion should be invalid without validation")
	}
}
```

### Integration Tests

```go
func TestBreakingChangeDetection_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name           string
		currentSchema  *Schema
		desiredSchema  *Schema
		expectedDangerous int
		expectedSafe   int
	}{
		{
			name: "Drop column with data",
			currentSchema: &Schema{
				Tables: []Table{
					{Name: "users", Columns: []Column{
						{Name: "id", Type: "BIGINT"},
						{Name: "last_login", Type: "TIMESTAMP"},
					}},
				},
			},
			desiredSchema: &Schema{
				Tables: []Table{
					{Name: "users", Columns: []Column{
						{Name: "id", Type: "BIGINT"},
					}},
				},
			},
			expectedDangerous: 1,
			expectedSafe: 0,
		},
		// More scenarios...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := DiffSchemas(tt.currentSchema, tt.desiredSchema)
			results := ValidateSchemaDiff(diff)

			dangerous := 0
			safe := 0
			for _, r := range results {
				if r.Safety != nil {
					switch r.Safety.Level {
					case SafetyLevelDangerous:
						dangerous++
					case SafetyLevelSafe:
						safe++
					}
				}
			}

			if dangerous != tt.expectedDangerous {
				t.Errorf("Expected %d dangerous, got %d", tt.expectedDangerous, dangerous)
			}
			if safe != tt.expectedSafe {
				t.Errorf("Expected %d safe, got %d", tt.expectedSafe, safe)
			}
		})
	}
}
```

---

## Documentation Plan

### 1. Migration Safety Guide (`docs/migration_safety.md`)

Topics to cover:
- Understanding safety classifications
- Breaking change catalog
- Safer alternatives for common patterns
- Multi-phase migration workflows
- Testing on shadow DB
- Production checklist

### 2. Update README

Add section on breaking change detection:
```markdown
## 🛡️ Breaking Change Detection

Lockplane automatically detects dangerous migrations and suggests safer alternatives:

- **❌ Dangerous**: Operations with permanent data loss (DROP COLUMN, DROP TABLE)
- **🔶 Lossy**: Operations where rollback loses data (ALTER TYPE, ADD NOT NULL)
- **⚠️ Requires Review**: Operations that might break apps (DROP INDEX, DROP FK)
- **✅ Safe**: Fully reversible, backward-compatible operations

### Example

```bash
$ lockplane plan --from-db --to schema/desired.json --validate

⚠️ Migration Safety Report:

Operation: DROP COLUMN users.last_login
Safety: ❌ Dangerous - Permanent data loss
Reversible: No

Safer Alternative:
  Use deprecation period:
    1. Stop writes (code deploy)
    2. Stop reads (code deploy)
    3. Drop column (safe)
```

### 3. CLI Help Text

The `lockplane plan --validate` command provides safety analysis:
```
--validate              Validate migration safety and reversibility
```

---

## Success Metrics

We succeed when:
1. ✅ All dangerous operations are flagged before production
2. ✅ Teams use safer alternatives (multi-phase patterns adopted)
3. ✅ Fewer production incidents from schema changes
4. ✅ Rollback safety is clear before migration
5. ✅ Shadow DB testing catches breaking changes

---

## Future Enhancements

After Phase 5:
- [ ] Lock duration estimation (query shadow DB)
- [ ] Data loss estimation (row counts, sizes)
- [ ] Automatic multi-phase plan generation
- [ ] pgroll integration for zero-downtime
- [ ] CI integration (block PR if dangerous)
- [ ] Configurable safety policies (strict/moderate/permissive)

---

## References

- Existing validation: `validation.go`
- Schema diff: `internal/schema/diff.go`
- Roadmap: `devdocs/roadmap.md` (Production Pain Point #7)
- Expand/contract pattern: Martin Fowler, refactoring.com
- pgroll: https://github.com/xataio/pgroll
