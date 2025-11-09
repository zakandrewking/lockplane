package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("failed to change back to original directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create test environments
	environments := []EnvironmentInput{
		{
			Name:         "local",
			DatabaseType: "postgres",
			Host:         "localhost",
			Port:         "5432",
			Database:     "testdb",
			User:         "testuser",
			Password:     "testpass",
		},
		{
			Name:         "staging",
			DatabaseType: "sqlite",
			FilePath:     "schema/staging.db",
		},
	}

	result, err := GenerateFiles(environments)
	if err != nil {
		t.Fatalf("GenerateFiles() error = %v", err)
	}

	// Verify result
	if !result.SchemaDirCreated {
		t.Error("expected schema directory to be created")
	}

	if !result.ConfigCreated {
		t.Error("expected config to be created")
	}

	if result.ConfigPath != "schema/lockplane.toml" {
		t.Errorf("expected config path to be 'schema/lockplane.toml', got %s", result.ConfigPath)
	}

	if len(result.EnvFiles) != 2 {
		t.Errorf("expected 2 env files, got %d", len(result.EnvFiles))
	}

	if !result.GitignoreUpdated {
		t.Error("expected gitignore to be updated")
	}

	// Verify files exist
	if _, err := os.Stat("schema"); os.IsNotExist(err) {
		t.Error("schema directory was not created")
	}

	if _, err := os.Stat("schema/lockplane.toml"); os.IsNotExist(err) {
		t.Error("lockplane.toml was not created")
	}

	if _, err := os.Stat(".env.local"); os.IsNotExist(err) {
		t.Error(".env.local was not created")
	}

	if _, err := os.Stat(".env.staging"); os.IsNotExist(err) {
		t.Error(".env.staging was not created")
	}

	if _, err := os.Stat(".gitignore"); os.IsNotExist(err) {
		t.Error(".gitignore was not created")
	}

	// Verify file contents
	configContent, err := os.ReadFile("schema/lockplane.toml")
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	configStr := string(configContent)
	if !strings.Contains(configStr, "default_environment = \"local\"") {
		t.Error("config should contain default_environment")
	}

	if !strings.Contains(configStr, "[environments.local]") {
		t.Error("config should contain local environment")
	}

	if !strings.Contains(configStr, "[environments.staging]") {
		t.Error("config should contain staging environment")
	}

	// Verify .env.local content
	envContent, err := os.ReadFile(".env.local")
	if err != nil {
		t.Fatalf("failed to read .env.local: %v", err)
	}

	envStr := string(envContent)
	if !strings.Contains(envStr, "DATABASE_URL=postgresql://testuser:testpass@localhost:5432/testdb") {
		t.Error(".env.local should contain PostgreSQL connection string")
	}

	if !strings.Contains(envStr, "SHADOW_DATABASE_URL=") {
		t.Error(".env.local should contain shadow database URL")
	}

	// Verify .env file permissions
	info, err := os.Stat(".env.local")
	if err != nil {
		t.Fatalf("failed to stat .env.local: %v", err)
	}

	perm := info.Mode().Perm()
	expectedPerm := os.FileMode(0600)
	if perm != expectedPerm {
		t.Errorf(".env.local permissions = %o, want %o", perm, expectedPerm)
	}

	// Verify .gitignore content
	gitignoreContent, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	gitignoreStr := string(gitignoreContent)
	if !strings.Contains(gitignoreStr, ".env.*") {
		t.Error(".gitignore should contain .env.* pattern")
	}
}

func TestUpdateGitignoreExisting(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("failed to change back to original directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create existing .gitignore
	existingContent := "*.log\nnode_modules/\n"
	if err := os.WriteFile(".gitignore", []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .gitignore: %v", err)
	}

	if err := updateGitignore(); err != nil {
		t.Fatalf("updateGitignore() error = %v", err)
	}

	content, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	contentStr := string(content)

	// Should preserve existing content
	if !strings.Contains(contentStr, "*.log") {
		t.Error(".gitignore should preserve existing content")
	}

	// Should add .env.* pattern
	if !strings.Contains(contentStr, ".env.*") {
		t.Error(".gitignore should contain .env.* pattern")
	}
}

func TestUpdateGitignoreAlreadyHasPattern(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("failed to change back to original directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create .gitignore that already has the pattern
	existingContent := "*.log\n.env.*\n"
	if err := os.WriteFile(".gitignore", []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .gitignore: %v", err)
	}

	if err := updateGitignore(); err != nil {
		t.Fatalf("updateGitignore() error = %v", err)
	}

	content, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	// Should not duplicate the pattern
	count := strings.Count(string(content), ".env.*")
	if count != 1 {
		t.Errorf(".env.* appears %d times, want 1", count)
	}
}

func TestGetEnvironmentNames(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "lockplane.toml")

	content := `default_environment = "local"

[environments.local]
description = "Local development"

[environments.staging]
description = "Staging environment"

[environments.production]
description = "Production"
`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	envNames, err := getEnvironmentNames(configPath)
	if err != nil {
		t.Fatalf("getEnvironmentNames() error = %v", err)
	}

	expected := []string{"local", "staging", "production"}
	if len(envNames) != len(expected) {
		t.Errorf("got %d environment names, want %d", len(envNames), len(expected))
	}

	for i, name := range expected {
		if i >= len(envNames) || envNames[i] != name {
			t.Errorf("environment[%d] = %q, want %q", i, envNames[i], name)
		}
	}
}

func TestGetEnvironmentNamesNonexistent(t *testing.T) {
	_, err := getEnvironmentNames("/nonexistent/path/lockplane.toml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}
