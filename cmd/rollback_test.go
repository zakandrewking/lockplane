package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRollbackCommandRegistered(t *testing.T) {
	cmd := findCommand(t, "rollback")

	if cmd.Short == "" {
		t.Error("rollbackCmd.Short should not be empty")
	}
	if cmd.Run == nil {
		t.Error("rollbackCmd.Run should not be nil")
	}
}

func TestRollbackCommandFlags(t *testing.T) {
	cmd := findCommand(t, "rollback")
	flags := cmd.Flags()

	expectedFlags := map[string]string{
		"plan":               "string",
		"from":               "string",
		"from-environment":   "string",
		"target":             "string",
		"target-environment": "string",
		"auto-approve":       "bool",
		"skip-shadow":        "bool",
		"shadow-db":          "string",
		"shadow-schema":      "string",
		"verbose":            "bool",
	}

	for name, flagType := range expectedFlags {
		flag := flags.Lookup(name)
		if flag == nil {
			t.Errorf("expected flag %q to exist", name)
			continue
		}
		if flag.Value.Type() != flagType {
			t.Errorf("expected flag %q to be %s, got %s", name, flagType, flag.Value.Type())
		}
	}
}

func TestPlanRollbackCommandFlags(t *testing.T) {
	cmd := findCommand(t, "plan-rollback")
	flags := cmd.Flags()

	expectedFlags := map[string]string{
		"plan":             "string",
		"from":             "string",
		"from-environment": "string",
		"verbose":          "bool",
	}

	for name, flagType := range expectedFlags {
		flag := flags.Lookup(name)
		if flag == nil {
			t.Errorf("expected flag %q to exist", name)
			continue
		}
		if flag.Value.Type() != flagType {
			t.Errorf("expected flag %q to be %s, got %s", name, flagType, flag.Value.Type())
		}
	}
}

func findCommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	t.Fatalf("command %q not registered", name)
	return nil
}
