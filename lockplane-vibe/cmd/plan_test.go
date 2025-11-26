package cmd

import (
	"testing"
)

func TestPlanCommand(t *testing.T) {
	if planCmd == nil {
		t.Fatal("planCmd should not be nil")
	}

	if planCmd.Use != "plan" {
		t.Errorf("expected Use to be 'plan', got %q", planCmd.Use)
	}

	if planCmd.Short == "" {
		t.Error("planCmd.Short should not be empty")
	}

	if planCmd.Long == "" {
		t.Error("planCmd.Long should not be empty")
	}

	if planCmd.Example == "" {
		t.Error("planCmd.Example should not be empty")
	}

	if planCmd.Run == nil {
		t.Error("planCmd.Run should not be nil")
	}
}

func TestPlanCommandFlags(t *testing.T) {
	flags := planCmd.Flags()

	// Test that required flags exist
	requiredFlags := []string{"from", "to", "from-environment", "to-environment", "check-schema", "verbose"}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

func TestPlanCommandFlagTypes(t *testing.T) {
	flags := planCmd.Flags()

	// Test string flags
	stringFlags := []string{"from", "to", "from-environment", "to-environment"}
	for _, flagName := range stringFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "string" {
			t.Errorf("expected flag %q to be of type string, got %s", flagName, flag.Value.Type())
		}
	}

	// Test boolean flags
	boolFlags := []string{"check-schema", "verbose"}
	for _, flagName := range boolFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "bool" {
			t.Errorf("expected flag %q to be of type bool, got %s", flagName, flag.Value.Type())
		}
	}
}

func TestPlanCommandVerboseShorthand(t *testing.T) {
	flag := planCmd.Flags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for verbose flag")
	}
}
