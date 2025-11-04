package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSchemaDirCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "schema")

	created, err := ensureSchemaDir(path)
	if err != nil {
		t.Fatalf("ensureSchemaDir returned error: %v", err)
	}
	if !created {
		t.Fatalf("expected directory to be created")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory to exist, err=%v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", path)
	}
}

func TestEnsureSchemaDirExistingDirectory(t *testing.T) {
	tmp := t.TempDir()

	created, err := ensureSchemaDir(tmp)
	if err != nil {
		t.Fatalf("expected existing directory to succeed, got %v", err)
	}
	if created {
		t.Fatalf("expected created=false when directory already exists")
	}
}

func TestEnsureSchemaDirExistingFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "schema")

	if err := os.WriteFile(file, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if created, err := ensureSchemaDir(file); err == nil {
		t.Fatalf("expected error when path is an existing file (created=%v)", created)
	}
}

func TestBootstrapSchemaDirectoryCreatesConfig(t *testing.T) {
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	result, err := bootstrapSchemaDirectory()
	if err != nil {
		t.Fatalf("bootstrapSchemaDirectory returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	if !result.ConfigCreated {
		t.Fatalf("expected config to be created")
	}

	configPath := filepath.Join(tmp, "schema", lockplaneConfigFilename)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", configPath, err)
	}
	if string(data) != defaultLockplaneTomlBody {
		t.Fatalf("unexpected config contents: %s", string(data))
	}
}

func TestBootstrapSchemaDirectoryErrorsWhenConfigExists(t *testing.T) {
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	schemaPath := filepath.Join(tmp, "schema")
	if err := os.MkdirAll(schemaPath, 0o755); err != nil {
		t.Fatalf("mkdir schema: %v", err)
	}
	configPath := filepath.Join(schemaPath, lockplaneConfigFilename)
	if err := os.WriteFile(configPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := bootstrapSchemaDirectory(); err == nil {
		t.Fatalf("expected error when config already exists")
	}
}
