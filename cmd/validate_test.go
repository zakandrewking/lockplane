package cmd

import (
	"testing"
)

func TestValidateCommand(t *testing.T) {
	if validateCmd == nil {
		t.Fatal("validateCmd should not be nil")
	}

	if validateCmd.Use != "validate" {
		t.Errorf("expected Use to be 'validate', got %q", validateCmd.Use)
	}

	if validateCmd.Short == "" {
		t.Error("validateCmd.Short should not be empty")
	}

	if validateCmd.Long == "" {
		t.Error("validateCmd.Long should not be empty")
	}

	if validateCmd.Example == "" {
		t.Error("validateCmd.Example should not be empty")
	}
}

func TestValidateSubcommands(t *testing.T) {
	subcommands := validateCmd.Commands()
	if len(subcommands) == 0 {
		t.Fatal("validate command should have subcommands")
	}

	expectedSubcommands := map[string]bool{
		"schema": false,
		"sql":    false,
		"plan":   false,
	}

	for _, cmd := range subcommands {
		if _, exists := expectedSubcommands[cmd.Name()]; exists {
			expectedSubcommands[cmd.Name()] = true
		}
	}

	for cmdName, found := range expectedSubcommands {
		if !found {
			t.Errorf("expected subcommand %q to exist under validate", cmdName)
		}
	}
}

func TestValidateSchemaCommand(t *testing.T) {
	if validateSchemaCmd == nil {
		t.Fatal("validateSchemaCmd should not be nil")
	}

	if validateSchemaCmd.Use != "schema [file]" {
		t.Errorf("expected Use to be 'schema [file]', got %q", validateSchemaCmd.Use)
	}

	if validateSchemaCmd.Run == nil {
		t.Error("validateSchemaCmd.Run should not be nil")
	}
}

func TestValidateSQLCommand(t *testing.T) {
	if validateSQLCmd == nil {
		t.Fatal("validateSQLCmd should not be nil")
	}

	if validateSQLCmd.Use != "sql [file]" {
		t.Errorf("expected Use to be 'sql [file]', got %q", validateSQLCmd.Use)
	}

	if validateSQLCmd.Run == nil {
		t.Error("validateSQLCmd.Run should not be nil")
	}
}

func TestValidatePlanCommand(t *testing.T) {
	if validatePlanCmd == nil {
		t.Fatal("validatePlanCmd should not be nil")
	}

	if validatePlanCmd.Use != "plan [file]" {
		t.Errorf("expected Use to be 'plan [file]', got %q", validatePlanCmd.Use)
	}

	if validatePlanCmd.Run == nil {
		t.Error("validatePlanCmd.Run should not be nil")
	}
}

func TestValidateSchemaFlags(t *testing.T) {
	flags := validateSchemaCmd.Flags()

	fileFlag := flags.Lookup("file")
	if fileFlag == nil {
		t.Error("expected --file flag to exist on validate schema command")
	}

	// Check shorthand
	if fileFlag != nil {
		shorthand := flags.ShorthandLookup("f")
		if shorthand == nil {
			t.Error("expected -f shorthand for file flag")
		}
	}
}

func TestValidateSQLFlags(t *testing.T) {
	flags := validateSQLCmd.Flags()

	outputFormatFlag := flags.Lookup("output-format")
	if outputFormatFlag == nil {
		t.Error("expected --output-format flag to exist on validate sql command")
	}
}
