package cmd

import (
	"testing"
)

func TestRootCommand(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}

	if rootCmd.Use != "lockplane" {
		t.Errorf("expected Use to be 'lockplane', got %q", rootCmd.Use)
	}

	if rootCmd.Short != "Postgres-first database schema management" {
		t.Errorf("expected Short description, got %q", rootCmd.Short)
	}
}

func TestVersionSet(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}

	if rootCmd.Version == "" {
		t.Error("rootCmd.Version should not be empty")
	}
}

func TestCommandsRegistered(t *testing.T) {
	commands := rootCmd.Commands()
	if len(commands) == 0 {
		t.Fatal("expected at least one subcommand to be registered")
	}

	expectedCommands := map[string]bool{
		"init":            false,
		"plan":            false,
		"apply":           false,
		"rollback":        false,
		"introspect":      false,
		"validate":        false,
		"convert":         false,
		"plan-multiphase": false,
		"apply-phase":     false,
		"rollback-phase":  false,
		"phase-status":    false,
	}

	for _, cmd := range commands {
		if _, exists := expectedCommands[cmd.Name()]; exists {
			expectedCommands[cmd.Name()] = true
		}
	}

	for cmdName, registered := range expectedCommands {
		if !registered {
			t.Errorf("expected command %q to be registered", cmdName)
		}
	}
}
