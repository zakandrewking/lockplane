package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestInitCommand(t *testing.T) {
	if initCmd == nil {
		t.Fatal("initCmd should not be nil")
	}

	if initCmd.Use != "init" {
		t.Errorf("expected Use to be 'init', got %q", initCmd.Use)
	}

	if initCmd.Short == "" {
		t.Error("initCmd.Short should not be empty")
	}

	if initCmd.Long == "" {
		t.Error("initCmd.Long should not be empty")
	}

	if initCmd.Run == nil {
		t.Error("initCmd.Run should not be nil")
	}
}

func TestCheckExistingConfig_NoConfig(t *testing.T) {
	switchToTempDir(t)

	// Test when no config exists
	result, err := checkExistingConfig()
	if err != nil {
		t.Errorf("checkExistingConfig should not return error when config doesn't exist: %v", err)
	}

	if result != nil {
		t.Errorf("expected nil result when config doesn't exist, got %v", result)
	}
}

func TestCheckExistingConfig_ConfigExists(t *testing.T) {
	switchToTempDir(t)

	if err := os.WriteFile(lockplaneConfigFilename, []byte("test config"), 0o644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test when config exists
	result, err := checkExistingConfig()
	if err != nil {
		t.Errorf("checkExistingConfig should not return error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result when config exists")
	}

	if *result != lockplaneConfigFilename {
		t.Errorf("expected result to be %q, got %q", lockplaneConfigFilename, *result)
	}
}

func TestReportBootstrapResult(t *testing.T) {
	var buf bytes.Buffer

	result := &bootstrapResult{
		SchemaDir:         "schema",
		ConfigPath:        "lockplane.toml",
		EnvFiles:          []string{".env.local"},
		SchemaDirCreated:  true,
		ConfigCreated:     true,
		GitignoreUpdated:  true,
		EnvExampleCreated: true,
	}

	reportBootstrapResult(&buf, result)

	out := buf.String()
	if !strings.Contains(out, "Created schema/") {
		t.Errorf("expected output to mention created schema dir, got:\n%s", out)
	}
	if !strings.Contains(out, "Next steps:") {
		t.Errorf("expected next steps section in output, got:\n%s", out)
	}
}

func TestReportBootstrapResult_Nil(t *testing.T) {
	var buf bytes.Buffer
	reportBootstrapResult(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil result, got %q", buf.String())
	}
}

func TestBootstrapResultUpdatedConfig(t *testing.T) {
	var buf bytes.Buffer

	result := &bootstrapResult{
		SchemaDir:     "schema",
		ConfigPath:    "lockplane.toml",
		EnvFiles:      []string{".env.local", ".env.production"},
		ConfigUpdated: true,
	}

	reportBootstrapResult(&buf, result)
	out := buf.String()
	if !strings.Contains(out, "Updated lockplane.toml") {
		t.Errorf("expected config update message in output, got:\n%s", out)
	}
}

func TestConstants(t *testing.T) {
	if defaultSchemaDir != "schema" {
		t.Errorf("expected defaultSchemaDir to be 'schema', got %q", defaultSchemaDir)
	}

	if lockplaneConfigFilename != "lockplane.toml" {
		t.Errorf("expected lockplaneConfigFilename to be 'lockplane.toml', got %q", lockplaneConfigFilename)
	}

	if !strings.Contains(defaultLockplaneTomlBody, "default_environment") {
		t.Error("defaultLockplaneTomlBody should contain 'default_environment'")
	}

	if !strings.Contains(defaultLockplaneTomlBody, "[environments.local]") {
		t.Error("defaultLockplaneTomlBody should contain '[environments.local]'")
	}
}

func switchToTempDir(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}
