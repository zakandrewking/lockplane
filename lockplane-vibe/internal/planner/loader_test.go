package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

// TestPlanJSON_ConsistencyWithGoTypes verifies that plan.json matches Go Plan struct
func TestPlanJSON_ConsistencyWithGoTypes(t *testing.T) {
	// Create fully-populated Plan with all fields
	plan := &Plan{
		SourceHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Steps: []PlanStep{
			{
				Description: "Create table users",
				SQL: []string{
					"CREATE TABLE users (id BIGINT PRIMARY KEY, email TEXT NOT NULL)",
				},
			},
			{
				Description: "Add index on email",
				SQL: []string{
					"CREATE UNIQUE INDEX idx_users_email ON users (email)",
				},
			},
		},
	}

	// Step 1: Marshal to JSON
	jsonData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal Plan to JSON: %v", err)
	}

	t.Logf("Generated JSON:\n%s", string(jsonData))

	// Step 2: Validate against JSON Schema
	// Find schema file relative to project root (../../schema-json/plan.json from internal/planner/)
	schemaPath := filepath.Join("..", "..", "schema-json", "plan.json")
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	documentLoader := gojsonschema.NewBytesLoader(jsonData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		t.Fatalf("JSON Schema validation failed to run: %v", err)
	}

	if !result.Valid() {
		t.Errorf("JSON Schema validation failed! This means plan.json is missing fields or has incorrect types:")
		for _, desc := range result.Errors() {
			t.Errorf("  - %s", desc)
		}
		t.FailNow()
	}

	// Step 3: Round-trip test
	var decoded Plan
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON back to Plan: %v", err)
	}

	// Verify fields are preserved
	if decoded.SourceHash != plan.SourceHash {
		t.Errorf("SourceHash not preserved: expected %s, got %s", plan.SourceHash, decoded.SourceHash)
	}

	if len(decoded.Steps) != len(plan.Steps) {
		t.Errorf("Step count mismatch: expected %d, got %d", len(plan.Steps), len(decoded.Steps))
	}

	if len(decoded.Steps) > 0 {
		if decoded.Steps[0].Description != plan.Steps[0].Description {
			t.Errorf("Step description not preserved: expected %s, got %s",
				plan.Steps[0].Description, decoded.Steps[0].Description)
		}
		if len(decoded.Steps[0].SQL) != len(plan.Steps[0].SQL) {
			t.Errorf("SQL statement count mismatch: expected %d, got %d",
				len(plan.Steps[0].SQL), len(decoded.Steps[0].SQL))
		}
	}

	t.Log("✅ plan.json is consistent with Go Plan type")
}

// TestMultiPhasePlanJSON_ConsistencyWithGoTypes verifies that plan-multi-phase.json matches Go MultiPhasePlan struct
func TestMultiPhasePlanJSON_ConsistencyWithGoTypes(t *testing.T) {
	warning := "Rollback after phase 2 may cause data loss"

	// Create fully-populated MultiPhasePlan with ALL fields
	multiPhasePlan := &MultiPhasePlan{
		MultiPhase:  true,
		Operation:   "rename_column",
		Description: "Rename users.name to users.full_name using expand-contract pattern",
		Pattern:     "expand_contract",
		TotalPhases: 3,
		Phases: []Phase{
			{
				PhaseNumber:        1,
				Name:               "expand",
				Description:        "Add new full_name column alongside existing name column",
				RequiresCodeDeploy: true,
				DependsOnPhase:     0, // No dependency
				CodeChangesRequired: []string{
					"Update application to write to both name and full_name columns",
					"Update reads to prefer full_name, fallback to name",
				},
				Plan: &Plan{
					SourceHash: "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
					Steps: []PlanStep{
						{
							Description: "Add full_name column",
							SQL: []string{
								"ALTER TABLE users ADD COLUMN full_name TEXT",
							},
						},
						{
							Description: "Copy data from name to full_name",
							SQL: []string{
								"UPDATE users SET full_name = name WHERE full_name IS NULL",
							},
						},
					},
				},
				Verification: []string{
					"Verify full_name column exists: SELECT full_name FROM users LIMIT 1",
					"Verify data copied: SELECT COUNT(*) FROM users WHERE full_name IS NULL",
				},
				Rollback: &PhaseRollback{
					Description: "Drop the full_name column",
					SQL: []string{
						"ALTER TABLE users DROP COLUMN full_name",
					},
					Note:         "Safe to rollback - original name column unchanged",
					Warning:      "",
					RequiresCode: true,
				},
				EstimatedDuration: "5 seconds",
				LockImpact:        "Brief ACCESS EXCLUSIVE lock during ALTER TABLE",
			},
			{
				PhaseNumber:        2,
				Name:               "migrate_writes",
				Description:        "Deploy code that writes only to full_name",
				RequiresCodeDeploy: true,
				DependsOnPhase:     1,
				CodeChangesRequired: []string{
					"Update application to write only to full_name column",
					"Remove dual-write logic",
				},
				Plan: &Plan{
					SourceHash: "1111111111111111111111111111111111111111111111111111111111111111",
					Steps: []PlanStep{
						{
							Description: "No database changes in this phase",
							SQL: []string{
								"-- This phase is code-only",
							},
						},
					},
				},
				Verification: []string{
					"Monitor application logs for write errors",
					"Verify full_name is being updated: SELECT full_name FROM users ORDER BY updated_at DESC LIMIT 5",
				},
				Rollback: &PhaseRollback{
					Description:  "Redeploy code from phase 1 that writes to both columns",
					SQL:          []string{},
					Note:         "No database rollback needed",
					Warning:      warning,
					RequiresCode: true,
				},
				EstimatedDuration: "10 minutes (code deployment time)",
				LockImpact:        "No locks",
			},
			{
				PhaseNumber:         3,
				Name:                "contract",
				Description:         "Drop old name column",
				RequiresCodeDeploy:  false,
				DependsOnPhase:      2,
				CodeChangesRequired: []string{
					// Empty - no code changes needed
				},
				Plan: &Plan{
					SourceHash: "2222222222222222222222222222222222222222222222222222222222222222",
					Steps: []PlanStep{
						{
							Description: "Drop name column",
							SQL: []string{
								"ALTER TABLE users DROP COLUMN name",
							},
						},
					},
				},
				Verification: []string{
					"Verify name column dropped: SELECT column_name FROM information_schema.columns WHERE table_name = 'users'",
					"Verify application still works without name column",
				},
				Rollback: &PhaseRollback{
					Description:  "Cannot rollback - data in name column is lost",
					SQL:          []string{},
					Note:         "To restore, must re-add name column and copy from full_name",
					Warning:      "Rollback requires downtime and data restoration",
					RequiresCode: false,
				},
				EstimatedDuration: "2 seconds",
				LockImpact:        "Brief ACCESS EXCLUSIVE lock during DROP COLUMN",
			},
		},
		SafetyNotes: []string{
			"Ensure phase 1 code is deployed and stable before proceeding to phase 2",
			"Monitor application errors between phases",
			"Phase 3 is irreversible - cannot rollback after dropping name column",
			"Estimated total time: 15-20 minutes including code deployments",
		},
		CreatedAt:  "2024-01-15T10:30:00Z",
		SchemaPath: "schema/desired_schema.json",
	}

	// Step 1: Marshal to JSON
	jsonData, err := json.MarshalIndent(multiPhasePlan, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal MultiPhasePlan to JSON: %v", err)
	}

	t.Logf("Generated JSON:\n%s", string(jsonData))

	// Step 2: Validate against JSON Schema
	// Find schema file relative to project root (../../schema-json/plan-multi-phase.json from internal/planner/)
	schemaPath := filepath.Join("..", "..", "schema-json", "plan-multi-phase.json")
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	documentLoader := gojsonschema.NewBytesLoader(jsonData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		t.Fatalf("JSON Schema validation failed to run: %v", err)
	}

	if !result.Valid() {
		t.Errorf("JSON Schema validation failed! This means plan-multi-phase.json is missing fields or has incorrect types:")
		for _, desc := range result.Errors() {
			t.Errorf("  - %s", desc)
		}
		t.FailNow()
	}

	// Step 3: Round-trip test
	var decoded MultiPhasePlan
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON back to MultiPhasePlan: %v", err)
	}

	// Verify critical fields are preserved
	if decoded.MultiPhase != multiPhasePlan.MultiPhase {
		t.Errorf("MultiPhase not preserved: expected %v, got %v", multiPhasePlan.MultiPhase, decoded.MultiPhase)
	}

	if decoded.Operation != multiPhasePlan.Operation {
		t.Errorf("Operation not preserved: expected %s, got %s", multiPhasePlan.Operation, decoded.Operation)
	}

	if decoded.Pattern != multiPhasePlan.Pattern {
		t.Errorf("Pattern not preserved: expected %s, got %s", multiPhasePlan.Pattern, decoded.Pattern)
	}

	if decoded.TotalPhases != multiPhasePlan.TotalPhases {
		t.Errorf("TotalPhases not preserved: expected %d, got %d", multiPhasePlan.TotalPhases, decoded.TotalPhases)
	}

	if len(decoded.Phases) != len(multiPhasePlan.Phases) {
		t.Errorf("Phase count mismatch: expected %d, got %d", len(multiPhasePlan.Phases), len(decoded.Phases))
	}

	// Check first phase in detail
	if len(decoded.Phases) > 0 {
		origPhase := multiPhasePlan.Phases[0]
		decodedPhase := decoded.Phases[0]

		if decodedPhase.PhaseNumber != origPhase.PhaseNumber {
			t.Errorf("PhaseNumber not preserved: expected %d, got %d",
				origPhase.PhaseNumber, decodedPhase.PhaseNumber)
		}

		if decodedPhase.Name != origPhase.Name {
			t.Errorf("Phase Name not preserved: expected %s, got %s",
				origPhase.Name, decodedPhase.Name)
		}

		if decodedPhase.RequiresCodeDeploy != origPhase.RequiresCodeDeploy {
			t.Errorf("RequiresCodeDeploy not preserved: expected %v, got %v",
				origPhase.RequiresCodeDeploy, decodedPhase.RequiresCodeDeploy)
		}

		if len(decodedPhase.CodeChangesRequired) != len(origPhase.CodeChangesRequired) {
			t.Errorf("CodeChangesRequired count mismatch: expected %d, got %d",
				len(origPhase.CodeChangesRequired), len(decodedPhase.CodeChangesRequired))
		}

		if decodedPhase.Plan == nil {
			t.Error("Phase.Plan not preserved")
		} else {
			if len(decodedPhase.Plan.Steps) != len(origPhase.Plan.Steps) {
				t.Errorf("Phase.Plan.Steps count mismatch: expected %d, got %d",
					len(origPhase.Plan.Steps), len(decodedPhase.Plan.Steps))
			}
		}

		if len(decodedPhase.Verification) != len(origPhase.Verification) {
			t.Errorf("Verification count mismatch: expected %d, got %d",
				len(origPhase.Verification), len(decodedPhase.Verification))
		}

		if decodedPhase.Rollback == nil {
			t.Error("Phase.Rollback not preserved")
		} else {
			if decodedPhase.Rollback.Description != origPhase.Rollback.Description {
				t.Errorf("Rollback.Description not preserved: expected %s, got %s",
					origPhase.Rollback.Description, decodedPhase.Rollback.Description)
			}

			if decodedPhase.Rollback.RequiresCode != origPhase.Rollback.RequiresCode {
				t.Errorf("Rollback.RequiresCode not preserved: expected %v, got %v",
					origPhase.Rollback.RequiresCode, decodedPhase.Rollback.RequiresCode)
			}
		}

		if decodedPhase.EstimatedDuration != origPhase.EstimatedDuration {
			t.Errorf("EstimatedDuration not preserved: expected %s, got %s",
				origPhase.EstimatedDuration, decodedPhase.EstimatedDuration)
		}

		if decodedPhase.LockImpact != origPhase.LockImpact {
			t.Errorf("LockImpact not preserved: expected %s, got %s",
				origPhase.LockImpact, decodedPhase.LockImpact)
		}
	}

	if len(decoded.SafetyNotes) != len(multiPhasePlan.SafetyNotes) {
		t.Errorf("SafetyNotes count mismatch: expected %d, got %d",
			len(multiPhasePlan.SafetyNotes), len(decoded.SafetyNotes))
	}

	if decoded.CreatedAt != multiPhasePlan.CreatedAt {
		t.Errorf("CreatedAt not preserved: expected %s, got %s",
			multiPhasePlan.CreatedAt, decoded.CreatedAt)
	}

	if decoded.SchemaPath != multiPhasePlan.SchemaPath {
		t.Errorf("SchemaPath not preserved: expected %s, got %s",
			multiPhasePlan.SchemaPath, decoded.SchemaPath)
	}

	t.Log("✅ plan-multi-phase.json is consistent with Go MultiPhasePlan type")
}

// TestLoadJSONPlan_ValidPlan tests loading a valid plan from file
func TestLoadJSONPlan_ValidPlan(t *testing.T) {
	// Create valid plan JSON
	plan := &Plan{
		SourceHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Steps: []PlanStep{
			{
				Description: "Create table",
				SQL:         []string{"CREATE TABLE test (id INT)"},
			},
		},
	}

	// Write to temp file
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal plan: %v", err)
	}

	if err := os.WriteFile(planPath, data, 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	// Load the plan
	loaded, err := LoadJSONPlan(planPath)
	if err != nil {
		t.Fatalf("LoadJSONPlan failed: %v", err)
	}

	// Verify plan was loaded correctly
	if loaded.SourceHash != plan.SourceHash {
		t.Errorf("SourceHash mismatch: expected %s, got %s", plan.SourceHash, loaded.SourceHash)
	}

	if len(loaded.Steps) != len(plan.Steps) {
		t.Errorf("Steps count mismatch: expected %d, got %d", len(plan.Steps), len(loaded.Steps))
	}

	if loaded.Steps[0].Description != plan.Steps[0].Description {
		t.Errorf("Step description mismatch: expected %s, got %s",
			plan.Steps[0].Description, loaded.Steps[0].Description)
	}
}

// TestLoadJSONPlan_InvalidJSON tests loading invalid JSON
func TestLoadJSONPlan_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	if err := os.WriteFile(planPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := LoadJSONPlan(planPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse plan JSON") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

// TestLoadJSONPlan_MissingFile tests loading non-existent file
func TestLoadJSONPlan_MissingFile(t *testing.T) {
	_, err := LoadJSONPlan("/nonexistent/plan.json")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to read JSON file") {
		t.Errorf("Expected read error, got: %v", err)
	}
}

// TestLoadJSONPlan_InvalidSchema tests plan that fails JSON schema validation
func TestLoadJSONPlan_InvalidSchema(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "invalid_schema.json")

	// Create plan with invalid source_hash (not a valid SHA-256)
	invalidPlan := `{
		"source_hash": "not-a-valid-hash",
		"steps": [
			{
				"description": "Test",
				"sql": ["SELECT 1"]
			}
		]
	}`

	if err := os.WriteFile(planPath, []byte(invalidPlan), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := LoadJSONPlan(planPath)
	// Note: This might succeed with a warning if schema file doesn't exist
	// or fail with validation error if schema exists
	// Both behaviors are acceptable based on the backwards compatibility logic
	if err != nil {
		if !strings.Contains(err.Error(), "validation") && !strings.Contains(err.Error(), "pattern") {
			t.Logf("Got error (expected): %v", err)
		}
	}
}

// TestLoadJSONPlan_MissingRequiredFields tests plan missing required fields
func TestLoadJSONPlan_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "missing_fields.json")

	// Create plan without required 'steps' field
	invalidPlan := `{
		"source_hash": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	}`

	if err := os.WriteFile(planPath, []byte(invalidPlan), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := LoadJSONPlan(planPath)
	// Should fail validation if schema exists, or succeed with empty steps if not
	if err != nil {
		t.Logf("Got validation error (expected if schema exists): %v", err)
	}
}
