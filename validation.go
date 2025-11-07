package main

import (
	"fmt"

	"github.com/lockplane/lockplane/internal/schema"
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

// AddForeignKeyValidator validates adding a new foreign key
type AddForeignKeyValidator struct {
	TableName    string
	ForeignKey   ForeignKey
	TargetSchema *Schema // The schema after the migration
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
	var refTable *Table
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

// ValidateAddedForeignKeys validates foreign keys being added to a table
func ValidateAddedForeignKeys(tableName string, foreignKeys []ForeignKey, targetSchema *Schema) []ValidationResult {
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
func ValidateSchemaDiffWithSchema(diff *schema.SchemaDiff, targetSchema *Schema) []ValidationResult {
	var results []ValidationResult

	// Validate added columns in modified tables
	for _, tableDiff := range diff.ModifiedTables {
		addedResults := ValidateAddedColumns(tableDiff.TableName, tableDiff.AddedColumns)
		results = append(results, addedResults...)

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

	// TODO: Validate modified columns, removed columns, removed tables, index changes, etc.

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
