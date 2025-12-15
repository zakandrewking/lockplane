package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunInit_ErrorHandling(t *testing.T) {
	// Check if we're being run as a subprocess
	if os.Getenv("TEST_RUN_INIT") == "1" {
		// This will run in the subprocess
		// Create a command with a flag that will cause wizard.Run to fail
		// (running in a directory where lockplane.toml already exists without --force)
		tmpDir := os.Getenv("TEST_TMPDIR")
		if tmpDir == "" {
			t.Fatal("TEST_TMPDIR not set")
		}

		// Change to temp directory
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change to temp directory: %v", err)
		}

		// Create a lockplane.toml to trigger an error
		if err := os.WriteFile("lockplane.toml", []byte("existing"), 0600); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// This should exit with code 1 because lockplane.toml already exists
		// and we're not using --force, but we ARE using --yes to skip interactive mode
		rootCmd.SetArgs([]string{"init", "--yes"})
		_ = rootCmd.Execute() // Error handling is done via os.Exit in runInit
		return
	}

	// Main test process - spawn subprocess
	tmpDir := t.TempDir()

	cmd := exec.Command(os.Args[0], "-test.run=TestRunInit_ErrorHandling")
	cmd.Env = append(os.Environ(), "TEST_RUN_INIT=1", "TEST_TMPDIR="+tmpDir)

	err := cmd.Run()

	// We expect the subprocess to exit with an error (non-zero exit code)
	if err == nil {
		t.Error("expected command to exit with error, but it succeeded")
		return
	}

	// Check that it's an exit error
	if exitError, ok := err.(*exec.ExitError); ok {
		// We expect exit code 1
		if exitError.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitError.ExitCode())
		}
	} else {
		t.Errorf("expected ExitError, got %v", err)
	}
}

// TestRunInit_Success tests that runInit succeeds in a clean directory
func TestRunInit_Success(t *testing.T) {
	// Check if we're being run as a subprocess
	if os.Getenv("TEST_RUN_INIT_SUCCESS") == "1" {
		// This will run in the subprocess
		tmpDir := os.Getenv("TEST_TMPDIR")
		if tmpDir == "" {
			t.Fatal("TEST_TMPDIR not set")
		}

		// Change to temp directory
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change to temp directory: %v", err)
		}

		// This should succeed and create lockplane.toml and schema/
		rootCmd.SetArgs([]string{"init", "--yes"})
		_ = rootCmd.Execute() // Error handling is done via os.Exit in runInit
		return
	}

	// Main test process - spawn subprocess
	tmpDir := t.TempDir()

	cmd := exec.Command(os.Args[0], "-test.run=TestRunInit_Success")
	cmd.Env = append(os.Environ(), "TEST_RUN_INIT_SUCCESS=1", "TEST_TMPDIR="+tmpDir)

	output, err := cmd.CombinedOutput()

	// We expect the subprocess to succeed
	if err != nil {
		t.Errorf("expected command to succeed, got error: %v\nOutput: %s", err, string(output))
	}

	// Verify files were created
	configPath := filepath.Join(tmpDir, "lockplane.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected lockplane.toml to be created")
	}

	schemaPath := filepath.Join(tmpDir, "schema")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Error("expected schema/ directory to be created")
	}

	examplePath := filepath.Join(tmpDir, "schema", "example.lp.sql")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Error("expected schema/example.lp.sql to be created")
	}
}
