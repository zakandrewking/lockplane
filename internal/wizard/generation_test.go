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

	if !result.EnvExampleCreated {
		t.Error("expected .env.example to be created")
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

	// Verify .env.example was created
	if _, err := os.Stat(".env.example"); os.IsNotExist(err) {
		t.Error(".env.example was not created")
	}

	// Verify .env.example content
	exampleContent, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	exampleStr := string(exampleContent)
	if !strings.Contains(exampleStr, "DATABASE_URL=") {
		t.Error(".env.example should contain DATABASE_URL")
	}

	if !strings.Contains(exampleStr, "SHADOW_DATABASE_URL=") {
		t.Error(".env.example should contain SHADOW_DATABASE_URL")
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

func TestGenerateFilesPreservesExistingEnvironments(t *testing.T) {
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

	// Step 1: Create initial config with local environment
	initialEnvs := []EnvironmentInput{
		{
			Name:         "local",
			Description:  "Local development",
			DatabaseType: "postgres",
			Host:         "localhost",
			Port:         "5432",
			Database:     "testdb",
			User:         "testuser",
			Password:     "testpass",
		},
	}

	result1, err := GenerateFiles(initialEnvs)
	if err != nil {
		t.Fatalf("GenerateFiles() first call error = %v", err)
	}

	if !result1.ConfigCreated {
		t.Error("expected config to be created on first call")
	}

	// Step 2: Add more environments (staging and production)
	newEnvs := []EnvironmentInput{
		{
			Name:         "staging",
			Description:  "Staging environment",
			DatabaseType: "postgres",
			Host:         "staging.example.com",
			Port:         "5432",
			Database:     "stagingdb",
			User:         "staginguser",
			Password:     "stagingpass",
		},
		{
			Name:         "production",
			Description:  "Production environment",
			DatabaseType: "postgres",
			Host:         "prod.example.com",
			Port:         "5432",
			Database:     "proddb",
			User:         "produser",
			Password:     "prodpass",
		},
	}

	result2, err := GenerateFiles(newEnvs)
	if err != nil {
		t.Fatalf("GenerateFiles() second call error = %v", err)
	}

	if !result2.ConfigUpdated {
		t.Error("expected config to be updated on second call")
	}

	// Step 3: Verify all environments exist in the config
	configContent, err := os.ReadFile("schema/lockplane.toml")
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	configStr := string(configContent)

	// Check that all three environments exist
	if !strings.Contains(configStr, "[environments.local]") {
		t.Error("config should preserve local environment")
	}

	if !strings.Contains(configStr, "[environments.staging]") {
		t.Error("config should contain new staging environment")
	}

	if !strings.Contains(configStr, "[environments.production]") {
		t.Error("config should contain new production environment")
	}

	// Verify descriptions
	if !strings.Contains(configStr, "Local development") {
		t.Error("config should preserve local environment description")
	}

	if !strings.Contains(configStr, "Staging environment") {
		t.Error("config should contain staging environment description")
	}

	if !strings.Contains(configStr, "Production environment") {
		t.Error("config should contain production environment description")
	}

	// Verify default environment is preserved (should be the first one from the initial setup)
	if !strings.Contains(configStr, "default_environment = \"local\"") {
		t.Error("config should preserve default_environment")
	}

	// Verify new .env files were created
	if _, err := os.Stat(".env.staging"); os.IsNotExist(err) {
		t.Error(".env.staging was not created")
	}

	if _, err := os.Stat(".env.production"); os.IsNotExist(err) {
		t.Error(".env.production was not created")
	}

	// Verify original .env.local still exists
	if _, err := os.Stat(".env.local"); os.IsNotExist(err) {
		t.Error(".env.local should still exist")
	}
}

func TestCreateOrUpdateEnvExampleNew(t *testing.T) {
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

	// Create .env.example from scratch
	if err := createOrUpdateEnvExample(); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "DATABASE_URL=") {
		t.Error(".env.example should contain DATABASE_URL")
	}

	if !strings.Contains(contentStr, "SHADOW_DATABASE_URL=") {
		t.Error(".env.example should contain SHADOW_DATABASE_URL")
	}

	if !strings.Contains(contentStr, "Lockplane") {
		t.Error(".env.example should contain Lockplane header")
	}
}

func TestCreateOrUpdateEnvExampleUpdate(t *testing.T) {
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

	// Create existing .env.example with only some content
	existingContent := "# Existing config\nSOME_VAR=value\n"
	if err := os.WriteFile(".env.example", []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .env.example: %v", err)
	}

	// Update .env.example
	if err := createOrUpdateEnvExample(); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was updated
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)

	// Should preserve existing content
	if !strings.Contains(contentStr, "SOME_VAR=value") {
		t.Error(".env.example should preserve existing content")
	}

	// Should add new content
	if !strings.Contains(contentStr, "DATABASE_URL=") {
		t.Error(".env.example should contain DATABASE_URL")
	}

	if !strings.Contains(contentStr, "SHADOW_DATABASE_URL=") {
		t.Error(".env.example should contain SHADOW_DATABASE_URL")
	}
}

func TestCreateOrUpdateEnvExampleIdempotent(t *testing.T) {
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

	// Create .env.example that already has both fields
	existingContent := "DATABASE_URL=postgres://localhost/db\nSHADOW_DATABASE_URL=postgres://localhost/shadow\n"
	if err := os.WriteFile(".env.example", []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .env.example: %v", err)
	}

	// Call update again
	if err := createOrUpdateEnvExample(); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was not modified
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)
	if contentStr != existingContent {
		t.Error(".env.example should not be modified when it already has both fields")
	}

	// Should not duplicate fields
	databaseURLCount := strings.Count(contentStr, "DATABASE_URL=")
	if databaseURLCount != 1 {
		t.Errorf("DATABASE_URL appears %d times, want 1", databaseURLCount)
	}

	shadowURLCount := strings.Count(contentStr, "SHADOW_DATABASE_URL=")
	if shadowURLCount != 1 {
		t.Errorf("SHADOW_DATABASE_URL appears %d times, want 1", shadowURLCount)
	}
}

func TestGenerateFilesUpdatesExistingEnvironment(t *testing.T) {
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

	// Step 1: Create initial config with local environment
	initialEnvs := []EnvironmentInput{
		{
			Name:         "local",
			Description:  "Local development",
			DatabaseType: "postgres",
		},
	}

	_, err = GenerateFiles(initialEnvs)
	if err != nil {
		t.Fatalf("GenerateFiles() first call error = %v", err)
	}

	// Step 2: Update the local environment with a new description
	updatedEnvs := []EnvironmentInput{
		{
			Name:         "local",
			Description:  "Updated local development environment",
			DatabaseType: "postgres",
		},
	}

	_, err = GenerateFiles(updatedEnvs)
	if err != nil {
		t.Fatalf("GenerateFiles() second call error = %v", err)
	}

	// Step 3: Verify the environment was updated
	configContent, err := os.ReadFile("schema/lockplane.toml")
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	configStr := string(configContent)

	// Check that the updated description exists
	if !strings.Contains(configStr, "Updated local development environment") {
		t.Error("config should contain updated description")
	}

	// Old description should not exist
	if strings.Contains(configStr, "description = \"Local development\"") {
		t.Error("config should not contain old description")
	}
}
