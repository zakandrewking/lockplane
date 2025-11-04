package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSchemaDirCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "schema")

	if err := ensureSchemaDir(path); err != nil {
		t.Fatalf("ensureSchemaDir returned error: %v", err)
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

	if err := ensureSchemaDir(tmp); err != nil {
		t.Fatalf("expected existing directory to succeed, got %v", err)
	}
}

func TestEnsureSchemaDirExistingFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "schema")

	if err := os.WriteFile(file, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := ensureSchemaDir(file); err == nil {
		t.Fatalf("expected error when path is an existing file")
	}
}
