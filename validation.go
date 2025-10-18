package main

import (
	"fmt"
)

// ValidationResult contains the outcome of validating a migration operation
type ValidationResult struct {
	Valid      bool     // Overall: can we safely do this migration?
	Reversible bool     // Can we generate a safe rollback?
	Errors     []string // Blocking issues
	Warnings   []string // Non-blocking concerns
	Reasons    []string // Why this validation passed/failed
}

// OperationValidator validates whether a diff operation is safe and reversible
type OperationValidator interface {
	Validate() ValidationResult
}

// AddColumnValidator validates adding a new column
type AddColumnValidator struct {
	TableName string
	Column    Column
}

func (v *AddColumnValidator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:      true,
		Reversible: true, // DROP COLUMN always reverses ADD COLUMN
		Errors:     []string{},
		Warnings:   []string{},
		Reasons:    []string{},
	}

	// Check if column is safe to add
	if !v.Column.Nullable && (v.Column.Default == nil || *v.Column.Default == "") {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Cannot add NOT NULL column '%s' without a DEFAULT value - existing rows would violate constraint",
				v.Column.Name))
		result.Reasons = append(result.Reasons,
			"NOT NULL columns require a DEFAULT value when added to tables with existing data")
	} else if v.Column.Nullable {
		result.Reasons = append(result.Reasons,
			fmt.Sprintf("Column '%s' is nullable - safe to add", v.Column.Name))
	} else if v.Column.Default != nil && *v.Column.Default != "" {
		result.Reasons = append(result.Reasons,
			fmt.Sprintf("Column '%s' has DEFAULT value - safe to add", v.Column.Name))
	}

	// Reversibility check
	result.Reasons = append(result.Reasons,
		fmt.Sprintf("Reversible: DROP COLUMN %s.%s", v.TableName, v.Column.Name))

	return result
}

// ValidateAddedColumns validates columns being added to a table
func ValidateAddedColumns(tableName string, columns []Column) []ValidationResult {
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

// ValidateSchemaDiff validates an entire schema diff
func ValidateSchemaDiff(diff *SchemaDiff) []ValidationResult {
	var results []ValidationResult

	// Validate added columns in modified tables
	for _, tableDiff := range diff.ModifiedTables {
		addedResults := ValidateAddedColumns(tableDiff.TableName, tableDiff.AddedColumns)
		results = append(results, addedResults...)
	}

	// TODO: Validate modified columns, removed columns, added tables, removed tables, index changes, etc.

	return results
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
