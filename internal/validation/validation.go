// Package validation provides schema validation and integrity checking.
//
// This package validates database schemas for correctness including foreign key
// references, duplicate names, missing primary keys, and other schema issues.
package validation

import (
	"fmt"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/schema"
)

// SafetyLevel represents how safe a migration operation is
type SafetyLevel int

const (
	SafetyLevelSafe       SafetyLevel = iota // ✅ Safe to apply, fully reversible
	SafetyLevelReview                        // ⚠️ Needs review, might be risky
	SafetyLevelLossy                         // 🔶 Lossy rollback, requires care
	SafetyLevelDangerous                     // ❌ Dangerous, permanent data loss
	SafetyLevelMultiPhase                    // 🔄 Requires multi-phase deployment
)

// String returns the human-readable name of the safety level
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

// Icon returns the emoji icon for the safety level
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

// SafetyClassification contains safety analysis for a migration operation
type SafetyClassification struct {
	Level               SafetyLevel // Overall safety level
	BreakingChange      bool        // Does this break running applications?
	DataLoss            bool        // Does this cause permanent data loss?
	RollbackDataLoss    bool        // Does rollback lose data?
	RequiresMultiPhase  bool        // Must be split into multiple migrations?
	LockContention      bool        // Will this hold heavyweight locks?
	RollbackDescription string      // What happens on rollback?
	SaferAlternatives   []string    // Suggested safer approaches
}

// ValidationResult contains the outcome of validating a migration operation
type ValidationResult struct {
	Valid      bool                  // Overall: can we safely do this migration?
	Reversible bool                  // Can we generate a safe rollback?
	Errors     []string              // Blocking issues
	Warnings   []string              // Non-blocking concerns
	Reasons    []string              // Why this validation passed/failed
	Safety     *SafetyClassification `json:"safety,omitempty"` // Safety analysis
}

// OperationValidator validates whether a diff operation is safe and reversible
type OperationValidator interface {
	Validate() ValidationResult
}

// AddColumnValidator validates adding a new column
type AddColumnValidator struct {
	TableName string
	Column    database.Column
}

func (v *AddColumnValidator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:      true,
		Reversible: true, // DROP COLUMN always reverses ADD COLUMN
		Errors:     []string{},
		Warnings:   []string{},
		Reasons:    []string{},
	}

	// Check if column is safe to add and classify safety
	if !v.Column.Nullable && (v.Column.Default == nil || *v.Column.Default == "") {
		// NOT NULL without DEFAULT - dangerous
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Cannot add NOT NULL column '%s' without a DEFAULT value - existing rows would violate constraint",
				v.Column.Name))
		result.Reasons = append(result.Reasons,
			"NOT NULL columns require a DEFAULT value when added to tables with existing data")

		result.Safety = &SafetyClassification{
			Level:               SafetyLevelDangerous,
			BreakingChange:      true,
			DataLoss:            false,
			RollbackDataLoss:    false,
			RequiresMultiPhase:  true,
			LockContention:      false,
			RollbackDescription: "Rollback will drop column, losing any data written to it",
			SaferAlternatives: []string{
				"Add column as nullable first",
				"Add column with DEFAULT value",
				"Use multi-phase: add nullable, backfill, make NOT NULL",
			},
		}
	} else if v.Column.Nullable {
		// Nullable column - safe
		result.Reasons = append(result.Reasons,
			fmt.Sprintf("Column '%s' is nullable - safe to add", v.Column.Name))

		result.Safety = &SafetyClassification{
			Level:               SafetyLevelSafe,
			BreakingChange:      false,
			DataLoss:            false,
			RollbackDataLoss:    true, // Rollback drops column with any written data
			RequiresMultiPhase:  false,
			LockContention:      false,
			RollbackDescription: "Rollback will drop column. Data written to this column will be lost.",
			SaferAlternatives:   []string{},
		}
	} else if v.Column.Default != nil && *v.Column.Default != "" {
		// NOT NULL with DEFAULT - safe
		result.Reasons = append(result.Reasons,
			fmt.Sprintf("Column '%s' has DEFAULT value - safe to add", v.Column.Name))

		result.Safety = &SafetyClassification{
			Level:               SafetyLevelSafe,
			BreakingChange:      false,
			DataLoss:            false,
			RollbackDataLoss:    true,
			RequiresMultiPhase:  false,
			LockContention:      false,
			RollbackDescription: "Rollback will drop column. Data written to this column will be lost.",
			SaferAlternatives:   []string{},
		}
	}

	// Reversibility check
	result.Reasons = append(result.Reasons,
		fmt.Sprintf("Reversible: DROP COLUMN %s.%s", v.TableName, v.Column.Name))

	return result
}

// AddForeignKeyValidator validates adding a new foreign key
type AddForeignKeyValidator struct {
	TableName    string
	ForeignKey   database.ForeignKey
	TargetSchema *database.Schema // The schema after the migration
}

func (v *AddForeignKeyValidator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:      true,
		Reversible: true, // DROP CONSTRAINT always reverses ADD CONSTRAINT
		Errors:     []string{},
		Warnings:   []string{},
		Reasons:    []string{},
	}

	// Check if referenced table exists
	var refTable *database.Table
	for i := range v.TargetSchema.Tables {
		if v.TargetSchema.Tables[i].Name == v.ForeignKey.ReferencedTable {
			refTable = &v.TargetSchema.Tables[i]
			break
		}
	}

	if refTable == nil {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Referenced table '%s' does not exist", v.ForeignKey.ReferencedTable))
		return result
	}

	// Check that column counts match
	if len(v.ForeignKey.Columns) != len(v.ForeignKey.ReferencedColumns) {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Foreign key column count (%d) does not match referenced column count (%d)",
				len(v.ForeignKey.Columns), len(v.ForeignKey.ReferencedColumns)))
		return result
	}

	// Check that referenced columns exist
	for i, refCol := range v.ForeignKey.ReferencedColumns {
		found := false
		for _, col := range refTable.Columns {
			if col.Name == refCol {
				found = true
				break
			}
		}
		if !found {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Referenced column '%s.%s' does not exist", v.ForeignKey.ReferencedTable, refCol))
		} else {
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("FK column '%s' → '%s.%s' is valid",
					v.ForeignKey.Columns[i], v.ForeignKey.ReferencedTable, refCol))
		}
	}

	// Reversibility check
	result.Reasons = append(result.Reasons,
		fmt.Sprintf("Reversible: DROP CONSTRAINT %s", v.ForeignKey.Name))

	return result
}

// ValidateAddedColumns validates columns being added to a table
func ValidateAddedColumns(tableName string, columns []database.Column) []ValidationResult {
	var results []ValidationResult
	for _, col := range columns {
		validator := &AddColumnValidator{
			TableName: tableName,
			Column:    col,
		}
		results = append(results, validator.Validate())
	}
	return results
}

// ValidateAddedForeignKeys validates foreign keys being added to a table
func ValidateAddedForeignKeys(tableName string, foreignKeys []database.ForeignKey, targetSchema *database.Schema) []ValidationResult {
	var results []ValidationResult
	for _, fk := range foreignKeys {
		validator := &AddForeignKeyValidator{
			TableName:    tableName,
			ForeignKey:   fk,
			TargetSchema: targetSchema,
		}
		results = append(results, validator.Validate())
	}
	return results
}

// ValidateSchemaDiff validates an entire schema diff
// It needs the target schema to validate foreign key references
func ValidateSchemaDiff(diff *schema.SchemaDiff) []ValidationResult {
	return ValidateSchemaDiffWithSchema(diff, nil)
}

// ValidateSchemaDiffWithSchema validates an entire schema diff with access to target schema
func ValidateSchemaDiffWithSchema(diff *schema.SchemaDiff, targetSchema *database.Schema) []ValidationResult {
	var results []ValidationResult

	// Validate removed tables (dangerous)
	for _, table := range diff.RemovedTables {
		validator := &DropTableValidator{
			Table: table,
			// TODO: Get row count from shadow DB analysis
		}
		results = append(results, validator.Validate())
	}

	// Validate modified tables
	for _, tableDiff := range diff.ModifiedTables {
		// Validate added columns
		addedResults := ValidateAddedColumns(tableDiff.TableName, tableDiff.AddedColumns)
		results = append(results, addedResults...)

		// Validate removed columns (dangerous)
		for _, col := range tableDiff.RemovedColumns {
			validator := &DropColumnValidator{
				TableName: tableDiff.TableName,
				Column:    col,
				// TODO: Get row count and column size from shadow DB analysis
			}
			results = append(results, validator.Validate())
		}

		// Validate modified columns (type changes)
		for _, colDiff := range tableDiff.ModifiedColumns {
			// Check if type changed
			typeChanged := false
			for _, change := range colDiff.Changes {
				if change == "type" {
					typeChanged = true
					break
				}
			}

			if typeChanged {
				validator := &AlterColumnTypeValidator{
					TableName:  tableDiff.TableName,
					ColumnName: colDiff.ColumnName,
					OldType:    colDiff.Old.Type,
					NewType:    colDiff.New.Type,
				}
				results = append(results, validator.Validate())
			}

			// TODO: Validate other column changes (nullable → NOT NULL, etc.)
		}

		// Validate RLS changes
		if tableDiff.RLSChanged {
			validator := &AlterRLSValidator{
				TableName: tableDiff.TableName,
				Enable:    tableDiff.RLSEnabled,
			}
			results = append(results, validator.Validate())
		}

		// Validate added foreign keys if we have the target schema
		if targetSchema != nil {
			fkResults := ValidateAddedForeignKeys(tableDiff.TableName, tableDiff.AddedForeignKeys, targetSchema)
			results = append(results, fkResults...)
		}
	}

	// Validate foreign keys in added tables
	if targetSchema != nil {
		for _, table := range diff.AddedTables {
			if len(table.ForeignKeys) > 0 {
				fkResults := ValidateAddedForeignKeys(table.Name, table.ForeignKeys, targetSchema)
				results = append(results, fkResults...)
			}
		}
	}

	return results
}

// DropColumnValidator validates dropping a column
type DropColumnValidator struct {
	TableName  string
	Column     database.Column
	RowCount   int64 // Optional: from shadow DB analysis
	ColumnSize int64 // Optional: estimated data loss in bytes
}

func (v *DropColumnValidator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:      true, // Valid but dangerous
		Reversible: false,
		Errors:     []string{},
		Warnings: []string{
			fmt.Sprintf("Dropping column '%s.%s' will permanently lose data",
				v.TableName, v.Column.Name),
		},
		Reasons: []string{
			"DROP COLUMN is irreversible - data cannot be recovered",
		},
		Safety: &SafetyClassification{
			Level:               SafetyLevelDangerous,
			BreakingChange:      true,
			DataLoss:            true,
			RollbackDataLoss:    false, // Can't rollback
			RequiresMultiPhase:  true,
			LockContention:      true, // Holds AccessExclusive lock
			RollbackDescription: "Cannot rollback - column data is permanently lost",
			SaferAlternatives: []string{
				"Use deprecation period: stop writes → archive data → stop reads → drop column",
				"Use expand/contract if renaming: add new column → dual-write → migrate reads → drop old",
			},
		},
	}

	if v.RowCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Estimated impact: %d rows", v.RowCount))
	}

	if v.ColumnSize > 0 {
		sizeMB := float64(v.ColumnSize) / (1024 * 1024)
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Estimated data loss: %.2f MB", sizeMB))
	}

	return result
}

// DropTableValidator validates dropping a table
type DropTableValidator struct {
	Table    database.Table
	RowCount int64 // Optional: from shadow DB analysis
}

func (v *DropTableValidator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:      true, // Valid but dangerous
		Reversible: false,
		Errors:     []string{},
		Warnings: []string{
			fmt.Sprintf("Dropping table '%s' will permanently lose all data", v.Table.Name),
		},
		Reasons: []string{
			"DROP TABLE is irreversible - all table data cannot be recovered",
		},
		Safety: &SafetyClassification{
			Level:               SafetyLevelDangerous,
			BreakingChange:      true,
			DataLoss:            true,
			RollbackDataLoss:    false, // Can't rollback
			RequiresMultiPhase:  true,
			LockContention:      true, // Holds AccessExclusive lock
			RollbackDescription: "Cannot rollback - all table data is permanently lost",
			SaferAlternatives: []string{
				"Use deprecation period: stop writes → archive data → stop reads → drop table",
				"Export table data to backup before dropping",
				"Rename table instead of drop, then drop later after verification",
			},
		},
	}

	if v.RowCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Estimated impact: %d rows will be lost", v.RowCount))
	}

	return result
}

// AlterColumnTypeValidator validates changing a column's type
type AlterColumnTypeValidator struct {
	TableName  string
	ColumnName string
	OldType    string
	NewType    string
}

func (v *AlterColumnTypeValidator) Validate() ValidationResult {
	// Analyze type conversion safety
	conversionSafe := isTypeConversionSafe(v.OldType, v.NewType)
	rollbackSafe := isTypeConversionSafe(v.NewType, v.OldType)

	var level SafetyLevel
	var alternatives []string
	var valid bool
	var warnings []string

	if !conversionSafe {
		level = SafetyLevelDangerous
		valid = false
		warnings = []string{
			fmt.Sprintf("Type conversion %s → %s might lose data or fail", v.OldType, v.NewType),
		}
		alternatives = []string{
			"Use multi-phase: add new column → backfill → dual-write → migrate reads → drop old",
			"Test conversion on shadow DB first to verify data compatibility",
			"Consider using a USING expression to handle conversion explicitly",
		}
	} else if !rollbackSafe {
		level = SafetyLevelLossy
		valid = true
		warnings = []string{
			fmt.Sprintf("Rollback will convert %s → %s, data might not fit", v.NewType, v.OldType),
		}
		alternatives = []string{
			"Test rollback on shadow DB to verify data fits old type",
			"Consider if this change is truly necessary",
		}
	} else {
		level = SafetyLevelSafe
		valid = true
	}

	return ValidationResult{
		Valid:      valid,
		Reversible: rollbackSafe,
		Errors:     []string{},
		Warnings:   warnings,
		Reasons: []string{
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

// AlterRLSValidator validates enabling or disabling row level security
type AlterRLSValidator struct {
	TableName string
	Enable    bool
}

func (v *AlterRLSValidator) Validate() ValidationResult {
	action := "Enable"
	rollbackAction := "disable"
	var saferAlternatives []string

	if v.Enable {
		saferAlternatives = []string{
			"Define row level security policies before enabling RLS.",
			"Test policies against staging/shadow databases to avoid lockouts.",
		}
	} else {
		action = "Disable"
		rollbackAction = "enable"
	}

	reason := fmt.Sprintf("%s row level security on table %s", action, v.TableName)
	rollback := fmt.Sprintf("Rollback will %s row level security on table %s", rollbackAction, v.TableName)

	return ValidationResult{
		Valid:      true,
		Reversible: true,
		Warnings:   []string{},
		Reasons: []string{
			reason,
		},
		Safety: &SafetyClassification{
			Level:              SafetyLevelSafe,
			BreakingChange:     false,
			DataLoss:           false,
			RollbackDataLoss:   false,
			RequiresMultiPhase: false,
			LockContention:     false,
			RollbackDescription: fmt.Sprintf(
				"%s.", rollback,
			),
			SaferAlternatives: saferAlternatives,
		},
	}
}

// isTypeConversionSafe checks if type conversion is safe (widening)
func isTypeConversionSafe(from, to string) bool {
	// Widening conversions (safe)
	safeConversions := map[string][]string{
		"SMALLINT":         {"INTEGER", "BIGINT", "NUMERIC", "DECIMAL"},
		"INTEGER":          {"BIGINT", "NUMERIC", "DECIMAL"},
		"BIGINT":           {"NUMERIC", "DECIMAL"},
		"REAL":             {"DOUBLE PRECISION", "NUMERIC", "DECIMAL"},
		"DOUBLE PRECISION": {"NUMERIC", "DECIMAL"},
		"VARCHAR":          {"TEXT"},
		"CHAR":             {"VARCHAR", "TEXT"},
		"DATE":             {"TIMESTAMP", "TIMESTAMPTZ"},
		"TIMESTAMP":        {"TIMESTAMPTZ"},
	}

	// Normalize types (uppercase, remove size constraints)
	from = normalizeType(from)
	to = normalizeType(to)

	// Check if same type (always safe)
	if from == to {
		return true
	}

	// Check if widening conversion
	if safe, ok := safeConversions[from]; ok {
		for _, safeType := range safe {
			if safeType == to {
				return true
			}
		}
	}

	return false
}

// normalizeType normalizes a SQL type for comparison
func normalizeType(typeName string) string {
	// Simple normalization: uppercase and remove size constraints
	// e.g., VARCHAR(255) → VARCHAR, INTEGER → INTEGER
	normalized := ""
	for _, ch := range typeName {
		if ch == '(' {
			break
		}
		if ch >= 'a' && ch <= 'z' {
			normalized += string(ch - 32) // Convert to uppercase
		} else if ch >= 'A' && ch <= 'Z' {
			normalized += string(ch)
		} else if ch == ' ' || ch == '_' {
			normalized += string(ch)
		}
	}
	return normalized
}

// AllValid returns true if all validation results are valid
func AllValid(results []ValidationResult) bool {
	for _, r := range results {
		if !r.Valid {
			return false
		}
	}
	return true
}

// AllReversible returns true if all operations are reversible
func AllReversible(results []ValidationResult) bool {
	for _, r := range results {
		if !r.Reversible {
			return false
		}
	}
	return true
}

// HasDangerousOperations returns true if any operation is dangerous
func HasDangerousOperations(results []ValidationResult) bool {
	for _, r := range results {
		if r.Safety != nil && r.Safety.Level == SafetyLevelDangerous {
			return true
		}
	}
	return false
}

// GetDangerousOperations returns all dangerous operation results
func GetDangerousOperations(results []ValidationResult) []ValidationResult {
	var dangerous []ValidationResult
	for _, r := range results {
		if r.Safety != nil && r.Safety.Level == SafetyLevelDangerous {
			dangerous = append(dangerous, r)
		}
	}
	return dangerous
}
