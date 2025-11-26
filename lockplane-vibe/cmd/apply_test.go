package cmd

import (
	"testing"
)

func TestApplyCommand(t *testing.T) {
	if applyCmd == nil {
		t.Fatal("applyCmd should not be nil")
	}

	if applyCmd.Use != "apply [plan.json]" {
		t.Errorf("expected Use to be 'apply [plan.json]', got %q", applyCmd.Use)
	}

	if applyCmd.Short == "" {
		t.Error("applyCmd.Short should not be empty")
	}

	if applyCmd.Long == "" {
		t.Error("applyCmd.Long should not be empty")
	}

	if applyCmd.Example == "" {
		t.Error("applyCmd.Example should not be empty")
	}

	if applyCmd.Run == nil {
		t.Error("applyCmd.Run should not be nil")
	}
}

func TestApplyCommandFlags(t *testing.T) {
	flags := applyCmd.Flags()

	requiredFlags := []string{
		"target",
		"target-environment",
		"schema",
		"auto-approve",
		"skip-shadow",
		"shadow-db",
		"shadow-schema",
		"verbose",
	}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

func TestApplyCommandFlagTypes(t *testing.T) {
	flags := applyCmd.Flags()

	// Test string flags
	stringFlags := []string{"target", "target-environment", "schema", "shadow-db", "shadow-schema"}
	for _, flagName := range stringFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "string" {
			t.Errorf("expected flag %q to be of type string, got %s", flagName, flag.Value.Type())
		}
	}

	// Test boolean flags
	boolFlags := []string{"auto-approve", "skip-shadow", "verbose"}
	for _, flagName := range boolFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "bool" {
			t.Errorf("expected flag %q to be of type bool, got %s", flagName, flag.Value.Type())
		}
	}
}

func TestApplyCommandVerboseShorthand(t *testing.T) {
	flag := applyCmd.Flags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for verbose flag")
	}
}
