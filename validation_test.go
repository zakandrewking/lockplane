package main

import (
	"testing"
)

func stringPtr(s string) *string {
	return &s
}

func TestAddColumnValidator_NullableColumn(t *testing.T) {
	validator := &AddColumnValidator{
		TableName: "users",
		Column: Column{
			Name:     "age",
			Type:     "integer",
			Nullable: true,
		},
	}

	result := validator.Validate()

	if !result.Valid {
		t.Errorf("Expected nullable column to be valid, got invalid with errors: %v", result.Errors)
	}

	if !result.Reversible {
		t.Error("Expected ADD COLUMN to be reversible")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors for nullable column, got: %v", result.Errors)
	}
}

func TestAddColumnValidator_NotNullWithDefault(t *testing.T) {
	validator := &AddColumnValidator{
		TableName: "users",
		Column: Column{
			Name:     "status",
			Type:     "text",
			Nullable: false,
			Default:  stringPtr("'active'"),
		},
	}

	result := validator.Validate()

	if !result.Valid {
		t.Errorf("Expected NOT NULL column with DEFAULT to be valid, got invalid with errors: %v", result.Errors)
	}

	if !result.Reversible {
		t.Error("Expected ADD COLUMN to be reversible")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

func TestAddColumnValidator_NotNullWithoutDefault(t *testing.T) {
	validator := &AddColumnValidator{
		TableName: "users",
		Column: Column{
			Name:     "email",
			Type:     "text",
			Nullable: false,
			Default:  nil,
		},
	}

	result := validator.Validate()

	if result.Valid {
		t.Error("Expected NOT NULL column without DEFAULT to be invalid")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for NOT NULL column without DEFAULT")
	}

	// Should still be reversible (we just can't apply it safely)
	if !result.Reversible {
		t.Error("Expected ADD COLUMN to be reversible even if not safe to apply")
	}
}

func TestAddColumnValidator_NotNullWithEmptyDefault(t *testing.T) {
	validator := &AddColumnValidator{
		TableName: "users",
		Column: Column{
			Name:     "email",
			Type:     "text",
			Nullable: false,
			Default:  stringPtr(""),
		},
	}

	result := validator.Validate()

	if result.Valid {
		t.Error("Expected NOT NULL column with empty DEFAULT to be invalid")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for NOT NULL column with empty DEFAULT")
	}
}

func TestValidateAddedColumns(t *testing.T) {
	columns := []Column{
		{
			Name:     "created_at",
			Type:     "timestamp",
			Nullable: true,
		},
		{
			Name:     "status",
			Type:     "text",
			Nullable: false,
			Default:  stringPtr("'active'"),
		},
	}

	results := ValidateAddedColumns("posts", columns)

	if len(results) != 2 {
		t.Errorf("Expected 2 validation results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Valid {
			t.Errorf("Expected result %d to be valid, got errors: %v", i, result.Errors)
		}
		if !result.Reversible {
			t.Errorf("Expected result %d to be reversible", i)
		}
	}
}

func TestValidateSchemaDiff_AddColumns(t *testing.T) {
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{
			{
				TableName: "users",
				AddedColumns: []Column{
					{
						Name:     "age",
						Type:     "integer",
						Nullable: true,
					},
					{
						Name:     "email",
						Type:     "text",
						Nullable: false,
						// Missing default - should fail
					},
				},
			},
		},
	}

	results := ValidateSchemaDiff(diff)

	if len(results) != 2 {
		t.Errorf("Expected 2 validation results, got %d", len(results))
	}

	if AllValid(results) {
		t.Error("Expected some validations to fail")
	}

	if !AllReversible(results) {
		t.Error("Expected all ADD COLUMN operations to be reversible")
	}

	// First one should pass
	if !results[0].Valid {
		t.Errorf("Expected first validation to pass, got errors: %v", results[0].Errors)
	}

	// Second one should fail
	if results[1].Valid {
		t.Error("Expected second validation to fail (NOT NULL without DEFAULT)")
	}
}

func TestValidateSchemaDiff_NoChanges(t *testing.T) {
	diff := &SchemaDiff{
		ModifiedTables: []TableDiff{},
	}

	results := ValidateSchemaDiff(diff)

	if len(results) != 0 {
		t.Errorf("Expected 0 validation results for empty diff, got %d", len(results))
	}

	if !AllValid(results) {
		t.Error("Expected empty results to be valid")
	}
}

func TestAllValid(t *testing.T) {
	results := []ValidationResult{
		{Valid: true, Reversible: true},
		{Valid: true, Reversible: true},
	}

	if !AllValid(results) {
		t.Error("Expected all validations to be valid")
	}

	results = append(results, ValidationResult{Valid: false, Reversible: true})

	if AllValid(results) {
		t.Error("Expected not all validations to be valid")
	}
}

func TestAllReversible(t *testing.T) {
	results := []ValidationResult{
		{Valid: true, Reversible: true},
		{Valid: false, Reversible: true},
	}

	if !AllReversible(results) {
		t.Error("Expected all operations to be reversible")
	}

	results = append(results, ValidationResult{Valid: true, Reversible: false})

	if AllReversible(results) {
		t.Error("Expected not all operations to be reversible")
	}
}
