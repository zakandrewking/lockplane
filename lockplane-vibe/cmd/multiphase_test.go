package cmd

import (
	"testing"

	"github.com/lockplane/lockplane/internal/planner"
)

func TestPlanMultiphaseCommand(t *testing.T) {
	if planMultiphaseCmd == nil {
		t.Fatal("planMultiphaseCmd should not be nil")
	}

	if planMultiphaseCmd.Use != "plan-multiphase" {
		t.Errorf("expected Use to be 'plan-multiphase', got %q", planMultiphaseCmd.Use)
	}

	if planMultiphaseCmd.Short == "" {
		t.Error("planMultiphaseCmd.Short should not be empty")
	}

	if planMultiphaseCmd.Long == "" {
		t.Error("planMultiphaseCmd.Long should not be empty")
	}

	if planMultiphaseCmd.Example == "" {
		t.Error("planMultiphaseCmd.Example should not be empty")
	}

	if planMultiphaseCmd.Run == nil {
		t.Error("planMultiphaseCmd.Run should not be nil")
	}
}

func TestPlanMultiphaseCommandFlags(t *testing.T) {
	flags := planMultiphaseCmd.Flags()

	requiredFlags := []string{
		"pattern",
		"table",
		"column",
		"old-column",
		"new-column",
		"type",
		"old-type",
		"new-type",
		"constraint",
		"source-hash",
		"archive-data",
	}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

func TestPlanMultiphaseCommandFlagTypes(t *testing.T) {
	flags := planMultiphaseCmd.Flags()

	// Test string flags
	stringFlags := []string{"pattern", "table", "column", "old-column", "new-column", "type", "old-type", "new-type", "constraint", "source-hash"}
	for _, flagName := range stringFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "string" {
			t.Errorf("expected flag %q to be of type string, got %s", flagName, flag.Value.Type())
		}
	}

	// Test boolean flags
	boolFlags := []string{"archive-data"}
	for _, flagName := range boolFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "bool" {
			t.Errorf("expected flag %q to be of type bool, got %s", flagName, flag.Value.Type())
		}
	}
}

func TestApplyPhaseCommand(t *testing.T) {
	if applyPhaseCmd == nil {
		t.Fatal("applyPhaseCmd should not be nil")
	}

	if applyPhaseCmd.Use != "apply-phase <plan-file>" {
		t.Errorf("expected Use to be 'apply-phase <plan-file>', got %q", applyPhaseCmd.Use)
	}

	if applyPhaseCmd.Short == "" {
		t.Error("applyPhaseCmd.Short should not be empty")
	}

	if applyPhaseCmd.Long == "" {
		t.Error("applyPhaseCmd.Long should not be empty")
	}

	if applyPhaseCmd.Example == "" {
		t.Error("applyPhaseCmd.Example should not be empty")
	}

	if applyPhaseCmd.Run == nil {
		t.Error("applyPhaseCmd.Run should not be nil")
	}

	if applyPhaseCmd.Args == nil {
		t.Error("applyPhaseCmd.Args should not be nil")
	}
}

func TestApplyPhaseCommandFlags(t *testing.T) {
	flags := applyPhaseCmd.Flags()

	requiredFlags := []string{
		"phase",
		"next",
		"force",
		"target-environment",
		"target",
		"shadow-db-environment",
		"shadow-db",
		"skip-shadow-db",
		"dry-run",
		"verbose",
		"auto-approve",
		"require-approval",
	}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

func TestApplyPhaseCommandFlagTypes(t *testing.T) {
	flags := applyPhaseCmd.Flags()

	// Test int flags
	intFlags := []string{"phase"}
	for _, flagName := range intFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "int" {
			t.Errorf("expected flag %q to be of type int, got %s", flagName, flag.Value.Type())
		}
	}

	// Test string flags
	stringFlags := []string{"target-environment", "target", "shadow-db-environment", "shadow-db"}
	for _, flagName := range stringFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "string" {
			t.Errorf("expected flag %q to be of type string, got %s", flagName, flag.Value.Type())
		}
	}

	// Test boolean flags
	boolFlags := []string{"next", "force", "skip-shadow-db", "dry-run", "verbose", "auto-approve", "require-approval"}
	for _, flagName := range boolFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "bool" {
			t.Errorf("expected flag %q to be of type bool, got %s", flagName, flag.Value.Type())
		}
	}
}

func TestApplyPhaseCommandVerboseShorthand(t *testing.T) {
	flag := applyPhaseCmd.Flags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for verbose flag")
	}
}

func TestRollbackPhaseCommand(t *testing.T) {
	if rollbackPhaseCmd == nil {
		t.Fatal("rollbackPhaseCmd should not be nil")
	}

	if rollbackPhaseCmd.Use != "rollback-phase <plan-file>" {
		t.Errorf("expected Use to be 'rollback-phase <plan-file>', got %q", rollbackPhaseCmd.Use)
	}

	if rollbackPhaseCmd.Short == "" {
		t.Error("rollbackPhaseCmd.Short should not be empty")
	}

	if rollbackPhaseCmd.Long == "" {
		t.Error("rollbackPhaseCmd.Long should not be empty")
	}

	if rollbackPhaseCmd.Example == "" {
		t.Error("rollbackPhaseCmd.Example should not be empty")
	}

	if rollbackPhaseCmd.Run == nil {
		t.Error("rollbackPhaseCmd.Run should not be nil")
	}

	if rollbackPhaseCmd.Args == nil {
		t.Error("rollbackPhaseCmd.Args should not be nil")
	}
}

func TestRollbackPhaseCommandFlags(t *testing.T) {
	flags := rollbackPhaseCmd.Flags()

	requiredFlags := []string{
		"phase",
		"force",
		"target-environment",
		"target",
		"dry-run",
		"verbose",
		"auto-approve",
	}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

func TestRollbackPhaseCommandFlagTypes(t *testing.T) {
	flags := rollbackPhaseCmd.Flags()

	// Test int flags
	intFlags := []string{"phase"}
	for _, flagName := range intFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "int" {
			t.Errorf("expected flag %q to be of type int, got %s", flagName, flag.Value.Type())
		}
	}

	// Test string flags
	stringFlags := []string{"target-environment", "target"}
	for _, flagName := range stringFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "string" {
			t.Errorf("expected flag %q to be of type string, got %s", flagName, flag.Value.Type())
		}
	}

	// Test boolean flags
	boolFlags := []string{"force", "dry-run", "verbose", "auto-approve"}
	for _, flagName := range boolFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "bool" {
			t.Errorf("expected flag %q to be of type bool, got %s", flagName, flag.Value.Type())
		}
	}
}

func TestRollbackPhaseCommandVerboseShorthand(t *testing.T) {
	flag := rollbackPhaseCmd.Flags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for verbose flag")
	}
}

func TestPhaseStatusCommand(t *testing.T) {
	if phaseStatusCmd == nil {
		t.Fatal("phaseStatusCmd should not be nil")
	}

	if phaseStatusCmd.Use != "phase-status" {
		t.Errorf("expected Use to be 'phase-status', got %q", phaseStatusCmd.Use)
	}

	if phaseStatusCmd.Short == "" {
		t.Error("phaseStatusCmd.Short should not be empty")
	}

	if phaseStatusCmd.Long == "" {
		t.Error("phaseStatusCmd.Long should not be empty")
	}

	if phaseStatusCmd.Example == "" {
		t.Error("phaseStatusCmd.Example should not be empty")
	}

	if phaseStatusCmd.Run == nil {
		t.Error("phaseStatusCmd.Run should not be nil")
	}
}

func TestPhaseStatusCommandFlags(t *testing.T) {
	flags := phaseStatusCmd.Flags()

	requiredFlags := []string{"verbose"}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

func TestPhaseStatusCommandFlagTypes(t *testing.T) {
	flags := phaseStatusCmd.Flags()

	// Test boolean flags
	boolFlags := []string{"verbose"}
	for _, flagName := range boolFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "bool" {
			t.Errorf("expected flag %q to be of type bool, got %s", flagName, flag.Value.Type())
		}
	}
}

func TestPhaseStatusCommandVerboseShorthand(t *testing.T) {
	flag := phaseStatusCmd.Flags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for verbose flag")
	}
}

func TestValidateMultiPhasePlan(t *testing.T) {
	tests := []struct {
		name    string
		plan    *planner.MultiPhasePlan
		wantErr bool
	}{
		{
			name: "valid plan",
			plan: &planner.MultiPhasePlan{
				MultiPhase:  true,
				TotalPhases: 2,
				Phases: []planner.Phase{
					{PhaseNumber: 1, Name: "Phase 1", Plan: &planner.Plan{}},
					{PhaseNumber: 2, Name: "Phase 2", Plan: &planner.Plan{}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid total phases",
			plan: &planner.MultiPhasePlan{
				MultiPhase:  true,
				TotalPhases: 0,
				Phases:      []planner.Phase{},
			},
			wantErr: true,
		},
		{
			name: "mismatched phase count",
			plan: &planner.MultiPhasePlan{
				MultiPhase:  true,
				TotalPhases: 2,
				Phases: []planner.Phase{
					{PhaseNumber: 1, Name: "Phase 1", Plan: &planner.Plan{}},
				},
			},
			wantErr: true,
		},
		{
			name: "incorrect phase number",
			plan: &planner.MultiPhasePlan{
				MultiPhase:  true,
				TotalPhases: 2,
				Phases: []planner.Phase{
					{PhaseNumber: 2, Name: "Phase 1", Plan: &planner.Plan{}},
					{PhaseNumber: 2, Name: "Phase 2", Plan: &planner.Plan{}},
				},
			},
			wantErr: true,
		},
		{
			name: "missing phase name",
			plan: &planner.MultiPhasePlan{
				MultiPhase:  true,
				TotalPhases: 1,
				Phases: []planner.Phase{
					{PhaseNumber: 1, Name: "", Plan: &planner.Plan{}},
				},
			},
			wantErr: true,
		},
		{
			name: "missing phase plan",
			plan: &planner.MultiPhasePlan{
				MultiPhase:  true,
				TotalPhases: 1,
				Phases: []planner.Phase{
					{PhaseNumber: 1, Name: "Phase 1", Plan: nil},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid dependency",
			plan: &planner.MultiPhasePlan{
				MultiPhase:  true,
				TotalPhases: 2,
				Phases: []planner.Phase{
					{PhaseNumber: 1, Name: "Phase 1", Plan: &planner.Plan{}},
					{PhaseNumber: 2, Name: "Phase 2", Plan: &planner.Plan{}, DependsOnPhase: 2},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMultiPhasePlan(tt.plan)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMultiPhasePlan() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
