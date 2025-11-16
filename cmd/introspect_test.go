package cmd

import (
	"testing"
)

func TestIntrospectCommand(t *testing.T) {
	if introspectCmd == nil {
		t.Fatal("introspectCmd should not be nil")
	}

	if introspectCmd.Use != "introspect" {
		t.Errorf("expected Use to be 'introspect', got %q", introspectCmd.Use)
	}

	if introspectCmd.Short == "" {
		t.Error("introspectCmd.Short should not be empty")
	}

	if introspectCmd.Long == "" {
		t.Error("introspectCmd.Long should not be empty")
	}

	if introspectCmd.Example == "" {
		t.Error("introspectCmd.Example should not be empty")
	}

	if introspectCmd.Run == nil {
		t.Error("introspectCmd.Run should not be nil")
	}
}

func TestIntrospectCommandFlags(t *testing.T) {
	flags := introspectCmd.Flags()

	requiredFlags := []string{
		"db",
		"format",
		"source-environment",
		"shadow",
		"verbose",
	}

	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

func TestIntrospectCommandFlagTypes(t *testing.T) {
	flags := introspectCmd.Flags()

	// Test string flags
	stringFlags := []string{"db", "format", "source-environment"}
	for _, flagName := range stringFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "string" {
			t.Errorf("expected flag %q to be of type string, got %s", flagName, flag.Value.Type())
		}
	}

	// Test boolean flags
	boolFlags := []string{"shadow", "verbose"}
	for _, flagName := range boolFlags {
		flag := flags.Lookup(flagName)
		if flag != nil && flag.Value.Type() != "bool" {
			t.Errorf("expected flag %q to be of type bool, got %s", flagName, flag.Value.Type())
		}
	}
}

func TestIntrospectVerboseShorthand(t *testing.T) {
	flag := introspectCmd.Flags().ShorthandLookup("v")
	if flag == nil {
		t.Error("expected -v shorthand for verbose flag")
	}
}
