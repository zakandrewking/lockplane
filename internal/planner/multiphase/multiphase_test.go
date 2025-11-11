package multiphase

import (
	"testing"
)

func TestGenerateExpandContractPlan(t *testing.T) {
	plan, err := GenerateExpandContractPlan("users", "email", "email_address", "TEXT", "abc123")
	if err != nil {
		t.Fatalf("Failed to generate expand/contract plan: %v", err)
	}

	if !plan.MultiPhase {
		t.Error("Expected MultiPhase to be true")
	}

	if plan.Operation != "rename_column" {
		t.Errorf("Expected operation 'rename_column', got '%s'", plan.Operation)
	}

	if plan.Pattern != "expand_contract" {
		t.Errorf("Expected pattern 'expand_contract', got '%s'", plan.Pattern)
	}

	if plan.TotalPhases != 3 {
		t.Errorf("Expected 3 phases, got %d", plan.TotalPhases)
	}

	if len(plan.Phases) != 3 {
		t.Fatalf("Expected 3 phases in array, got %d", len(plan.Phases))
	}

	// Check Phase 1: Expand
	phase1 := plan.Phases[0]
	if phase1.PhaseNumber != 1 {
		t.Errorf("Phase 1: expected PhaseNumber 1, got %d", phase1.PhaseNumber)
	}
	if phase1.Name != "expand" {
		t.Errorf("Phase 1: expected name 'expand', got '%s'", phase1.Name)
	}
	if !phase1.RequiresCodeDeploy {
		t.Error("Phase 1: expected RequiresCodeDeploy true")
	}
	if len(phase1.Plan.Steps) != 2 {
		t.Errorf("Phase 1: expected 2 steps, got %d", len(phase1.Plan.Steps))
	}

	// Check Phase 2: Migrate Reads
	phase2 := plan.Phases[1]
	if phase2.PhaseNumber != 2 {
		t.Errorf("Phase 2: expected PhaseNumber 2, got %d", phase2.PhaseNumber)
	}
	if phase2.Name != "migrate_reads" {
		t.Errorf("Phase 2: expected name 'migrate_reads', got '%s'", phase2.Name)
	}
	if !phase2.RequiresCodeDeploy {
		t.Error("Phase 2: expected RequiresCodeDeploy true")
	}
	if phase2.DependsOnPhase != 1 {
		t.Errorf("Phase 2: expected DependsOnPhase 1, got %d", phase2.DependsOnPhase)
	}
	if len(phase2.Plan.Steps) != 0 {
		t.Errorf("Phase 2: expected 0 steps (code only), got %d", len(phase2.Plan.Steps))
	}

	// Check Phase 3: Contract
	phase3 := plan.Phases[2]
	if phase3.PhaseNumber != 3 {
		t.Errorf("Phase 3: expected PhaseNumber 3, got %d", phase3.PhaseNumber)
	}
	if phase3.Name != "contract" {
		t.Errorf("Phase 3: expected name 'contract', got '%s'", phase3.Name)
	}
	if phase3.DependsOnPhase != 2 {
		t.Errorf("Phase 3: expected DependsOnPhase 2, got %d", phase3.DependsOnPhase)
	}
	if len(phase3.Plan.Steps) != 1 {
		t.Errorf("Phase 3: expected 1 step, got %d", len(phase3.Plan.Steps))
	}
}

func TestGenerateExpandContractPlan_Validation(t *testing.T) {
	// Test missing table
	_, err := GenerateExpandContractPlan("", "email", "email_address", "TEXT", "abc123")
	if err == nil {
		t.Error("Expected error for missing table")
	}

	// Test same column names
	_, err = GenerateExpandContractPlan("users", "email", "email", "TEXT", "abc123")
	if err == nil {
		t.Error("Expected error for same column names")
	}
}

func TestGenerateDeprecationPlan(t *testing.T) {
	// Without archive
	plan, err := GenerateDeprecationPlan("users", "old_column", "TEXT", false, "abc123")
	if err != nil {
		t.Fatalf("Failed to generate deprecation plan: %v", err)
	}

	if plan.Pattern != "deprecation" {
		t.Errorf("Expected pattern 'deprecation', got '%s'", plan.Pattern)
	}

	if plan.TotalPhases != 3 {
		t.Errorf("Expected 3 phases (no archive), got %d", plan.TotalPhases)
	}

	// With archive
	planWithArchive, err := GenerateDeprecationPlan("users", "old_column", "TEXT", true, "abc123")
	if err != nil {
		t.Fatalf("Failed to generate deprecation plan with archive: %v", err)
	}

	if planWithArchive.TotalPhases != 4 {
		t.Errorf("Expected 4 phases (with archive), got %d", planWithArchive.TotalPhases)
	}

	// Check phase names
	expectedPhases := []string{"stop_writes", "archive", "stop_reads", "drop_column"}
	for i, expectedName := range expectedPhases {
		if i >= len(planWithArchive.Phases) {
			t.Errorf("Missing phase %d", i+1)
			continue
		}
		if planWithArchive.Phases[i].Name != expectedName {
			t.Errorf("Phase %d: expected name '%s', got '%s'", i+1, expectedName, planWithArchive.Phases[i].Name)
		}
	}
}

func TestGenerateValidationPhasePlan_NotNull(t *testing.T) {
	plan, err := GenerateValidationPhasePlan(
		"users",
		"email",
		"TEXT",
		"not_null",
		"'placeholder@example.com'",
		"",
		"abc123",
	)
	if err != nil {
		t.Fatalf("Failed to generate validation plan for NOT NULL: %v", err)
	}

	if plan.Pattern != "validation" {
		t.Errorf("Expected pattern 'validation', got '%s'", plan.Pattern)
	}

	if plan.TotalPhases != 4 {
		t.Errorf("Expected 4 phases for NOT NULL, got %d", plan.TotalPhases)
	}

	// Check phase names
	expectedPhases := []string{"backfill", "add_constraint_not_valid", "validate", "make_not_null"}
	for i, expectedName := range expectedPhases {
		if plan.Phases[i].Name != expectedName {
			t.Errorf("Phase %d: expected name '%s', got '%s'", i+1, expectedName, plan.Phases[i].Name)
		}
	}
}

func TestGenerateValidationPhasePlan_Check(t *testing.T) {
	plan, err := GenerateValidationPhasePlan(
		"products",
		"price",
		"NUMERIC",
		"check",
		"",
		"price >= 0",
		"abc123",
	)
	if err != nil {
		t.Fatalf("Failed to generate validation plan for CHECK: %v", err)
	}

	if plan.TotalPhases != 3 {
		t.Errorf("Expected 3 phases for CHECK, got %d", plan.TotalPhases)
	}
}

func TestGenerateValidationPhasePlan_Unique(t *testing.T) {
	plan, err := GenerateValidationPhasePlan(
		"users",
		"username",
		"TEXT",
		"unique",
		"",
		"",
		"abc123",
	)
	if err != nil {
		t.Fatalf("Failed to generate validation plan for UNIQUE: %v", err)
	}

	if plan.TotalPhases != 3 {
		t.Errorf("Expected 3 phases for UNIQUE, got %d", plan.TotalPhases)
	}
}

func TestGenerateValidationPhasePlan_Validation(t *testing.T) {
	// Missing backfill value for NOT NULL
	_, err := GenerateValidationPhasePlan("users", "email", "TEXT", "not_null", "", "", "abc123")
	if err == nil {
		t.Error("Expected error for missing backfill value")
	}

	// Invalid constraint type
	_, err = GenerateValidationPhasePlan("users", "email", "TEXT", "invalid", "value", "", "abc123")
	if err == nil {
		t.Error("Expected error for invalid constraint type")
	}

	// Missing check expression for CHECK constraint
	_, err = GenerateValidationPhasePlan("users", "price", "NUMERIC", "check", "", "", "abc123")
	if err == nil {
		t.Error("Expected error for missing check expression")
	}
}

func TestGenerateTypeChangePlan(t *testing.T) {
	plan, err := GenerateTypeChangePlan(
		"users",
		"age",
		"INTEGER",
		"BIGINT",
		"CAST(age AS BIGINT)",
		"abc123",
	)
	if err != nil {
		t.Fatalf("Failed to generate type change plan: %v", err)
	}

	if plan.Pattern != "type_change" {
		t.Errorf("Expected pattern 'type_change', got '%s'", plan.Pattern)
	}

	if plan.TotalPhases != 5 {
		t.Errorf("Expected 5 phases, got %d", plan.TotalPhases)
	}

	// Check phase names
	expectedPhases := []string{"add_new_column", "enable_dual_write", "backfill", "migrate_reads", "drop_old_column"}
	for i, expectedName := range expectedPhases {
		if plan.Phases[i].Name != expectedName {
			t.Errorf("Phase %d: expected name '%s', got '%s'", i+1, expectedName, plan.Phases[i].Name)
		}
	}

	// Verify dependencies
	for i := 1; i < len(plan.Phases); i++ {
		if plan.Phases[i].DependsOnPhase != i {
			t.Errorf("Phase %d: expected DependsOnPhase %d, got %d", i+1, i, plan.Phases[i].DependsOnPhase)
		}
	}
}

func TestGenerateTypeChangePlan_DefaultConversion(t *testing.T) {
	// Test with empty conversion expression - should use default CAST
	plan, err := GenerateTypeChangePlan(
		"users",
		"age",
		"INTEGER",
		"BIGINT",
		"", // Empty conversion expression
		"abc123",
	)
	if err != nil {
		t.Fatalf("Failed to generate type change plan with default conversion: %v", err)
	}

	// Check that backfill step uses default CAST
	backfillPhase := plan.Phases[2] // Phase 3: backfill
	if len(backfillPhase.Plan.Steps) != 1 {
		t.Fatalf("Expected 1 backfill step, got %d", len(backfillPhase.Plan.Steps))
	}

	backfillSQL := backfillPhase.Plan.Steps[0].SQL[0]
	if backfillSQL == "" {
		t.Error("Backfill SQL should not be empty")
	}
}

func TestGenerateTypeChangePlan_Validation(t *testing.T) {
	// Missing required fields
	_, err := GenerateTypeChangePlan("", "age", "INTEGER", "BIGINT", "", "abc123")
	if err == nil {
		t.Error("Expected error for missing table")
	}

	_, err = GenerateTypeChangePlan("users", "", "INTEGER", "BIGINT", "", "abc123")
	if err == nil {
		t.Error("Expected error for missing column")
	}

	_, err = GenerateTypeChangePlan("users", "age", "", "BIGINT", "", "abc123")
	if err == nil {
		t.Error("Expected error for missing oldType")
	}

	_, err = GenerateTypeChangePlan("users", "age", "INTEGER", "", "", "abc123")
	if err == nil {
		t.Error("Expected error for missing newType")
	}
}
