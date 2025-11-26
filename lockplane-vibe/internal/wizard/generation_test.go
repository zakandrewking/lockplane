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

	if result.ConfigPath != "lockplane.toml" {
		t.Errorf("expected config path to be 'lockplane.toml', got %s", result.ConfigPath)
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

	if _, err := os.Stat("lockplane.toml"); os.IsNotExist(err) {
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
	configContent, err := os.ReadFile("lockplane.toml")
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	configStr := string(configContent)
	if !strings.Contains(configStr, "default_environment = \"local\"") {
		t.Error("config should contain default_environment")
	}

	if !strings.Contains(configStr, "schema_path = \"schema\"") {
		t.Error("config should set schema_path when missing")
	}

	if !strings.Contains(configStr, "dialect = \"postgres\"") {
		t.Error("config should contain global dialect")
	}

	if !strings.Contains(configStr, "schemas = [\"public\"]") {
		t.Error("config should contain default schema list")
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
	if !strings.Contains(envStr, "POSTGRES_URL=postgresql://testuser:testpass@localhost:5432/testdb") {
		t.Error(".env.local should contain PostgreSQL connection string")
	}

	if !strings.Contains(envStr, "POSTGRES_SHADOW_URL=") {
		t.Error(".env.local should contain PostgreSQL shadow database URL")
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
	// Should contain PostgreSQL variables (first environment)
	if !strings.Contains(exampleStr, "POSTGRES_URL=") {
		t.Error(".env.example should contain POSTGRES_URL")
	}

	if !strings.Contains(exampleStr, "POSTGRES_SHADOW_URL=") {
		t.Error(".env.example should contain POSTGRES_SHADOW_URL")
	}

	if !strings.Contains(exampleStr, "postgresql://") {
		t.Error(".env.example should contain PostgreSQL examples")
	}

	// Should contain SQLite variables (second environment)
	if !strings.Contains(exampleStr, "SQLITE_DB_PATH=") {
		t.Error(".env.example should contain SQLITE_DB_PATH")
	}

	if !strings.Contains(exampleStr, "SQLITE_SHADOW_DB_PATH=") {
		t.Error(".env.example should contain SQLITE_SHADOW_DB_PATH")
	}

	// Should NOT contain libSQL variables since no libsql environment was created
	if strings.Contains(exampleStr, "LIBSQL_") {
		t.Error(".env.example should not contain LIBSQL variables when no libsql environment is configured")
	}
}

func TestGenerateFilesCustomSchemaPath(t *testing.T) {
	tmpDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	envs := []EnvironmentInput{
		{
			Name:         "supabase",
			DatabaseType: "postgres",
			Host:         "127.0.0.1",
			Port:         "54322",
			Database:     "postgres",
			User:         "postgres",
			Password:     "postgres",
			SchemaPath:   "supabase/schema",
		},
	}

	result, err := GenerateFiles(envs)
	if err != nil {
		t.Fatalf("GenerateFiles returned error: %v", err)
	}

	if result.SchemaDir != "supabase/schema" {
		t.Fatalf("expected schema dir supabase/schema, got %q", result.SchemaDir)
	}
	if _, err := os.Stat("supabase/schema"); err != nil {
		t.Fatalf("supabase/schema directory missing: %v", err)
	}

	configBytes, err := os.ReadFile("lockplane.toml")
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	configStr := string(configBytes)
	if !strings.Contains(configStr, "schema_path = \"supabase/schema\"") {
		t.Fatalf("config missing supabase schema_path:\n%s", configStr)
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
	configContent, err := os.ReadFile("lockplane.toml")
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

	// Create .env.example from scratch with a postgres environment
	envs := []EnvironmentInput{
		{
			Name:         "development",
			DatabaseType: "postgres",
		},
	}
	if err := createOrUpdateEnvExample(envs); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "POSTGRES_URL=postgresql://") {
		t.Error(".env.example should contain POSTGRES_URL")
	}

	if !strings.Contains(contentStr, "POSTGRES_SHADOW_URL=postgresql://") {
		t.Error(".env.example should contain POSTGRES_SHADOW_URL")
	}

	if !strings.Contains(contentStr, "Lockplane") {
		t.Error(".env.example should contain Lockplane header")
	}

	// Should NOT contain libSQL or SQLite variables for postgres environment
	if strings.Contains(contentStr, "LIBSQL_") {
		t.Error(".env.example should not contain LIBSQL variables for postgres environment")
	}
	if strings.Contains(contentStr, "SQLITE_") {
		t.Error(".env.example should not contain SQLITE variables for postgres environment")
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

	// Create existing .env.example with PostgreSQL variables already set
	existingContent := "# Existing config\nPOSTGRES_URL=postgresql://existing\nPOSTGRES_SHADOW_URL=postgresql://existing_shadow\n"
	if err := os.WriteFile(".env.example", []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .env.example: %v", err)
	}

	// Add a libsql environment - should append libSQL variables
	envsWithLibSQL := []EnvironmentInput{
		{
			Name:         "turso",
			DatabaseType: "libsql",
		},
	}
	if err := createOrUpdateEnvExample(envsWithLibSQL); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was updated
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)

	// Should preserve existing content
	if !strings.Contains(contentStr, "POSTGRES_URL=postgresql://existing") {
		t.Error(".env.example should preserve existing POSTGRES_URL")
	}

	if !strings.Contains(contentStr, "POSTGRES_SHADOW_URL=postgresql://existing_shadow") {
		t.Error(".env.example should preserve existing POSTGRES_SHADOW_URL")
	}

	// Should add libSQL variables for libsql environment
	if !strings.Contains(contentStr, "LIBSQL_URL=") {
		t.Error(".env.example should contain LIBSQL_URL when libsql environment is added")
	}
	if !strings.Contains(contentStr, "LIBSQL_AUTH_TOKEN=") {
		t.Error(".env.example should contain LIBSQL_AUTH_TOKEN when libsql environment is added")
	}
	if !strings.Contains(contentStr, "LIBSQL_SHADOW_DB_PATH=") {
		t.Error(".env.example should contain LIBSQL_SHADOW_DB_PATH when libsql environment is added")
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

	// Create .env.example that already has all fields for libSQL
	existingContent := "LIBSQL_URL=libsql://localhost/db\nLIBSQL_AUTH_TOKEN=token123\nLIBSQL_SHADOW_DB_PATH=./shadow.db\n"
	if err := os.WriteFile(".env.example", []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create .env.example: %v", err)
	}

	// Call update again with libsql environment
	envsWithLibSQL := []EnvironmentInput{
		{
			Name:         "turso",
			DatabaseType: "libsql",
		},
	}
	if err := createOrUpdateEnvExample(envsWithLibSQL); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was not modified
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)
	if contentStr != existingContent {
		t.Errorf(".env.example should not be modified when it already has all fields\nExpected:\n%s\nGot:\n%s", existingContent, contentStr)
	}

	// Should not duplicate fields
	lines := strings.Split(contentStr, "\n")
	libsqlURLCount := 0
	libsqlTokenCount := 0
	libsqlShadowCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "LIBSQL_URL=") {
			libsqlURLCount++
		}
		if strings.HasPrefix(line, "LIBSQL_AUTH_TOKEN=") {
			libsqlTokenCount++
		}
		if strings.HasPrefix(line, "LIBSQL_SHADOW_DB_PATH=") {
			libsqlShadowCount++
		}
	}

	if libsqlURLCount != 1 {
		t.Errorf("LIBSQL_URL appears %d times, want 1", libsqlURLCount)
	}

	if libsqlTokenCount != 1 {
		t.Errorf("LIBSQL_AUTH_TOKEN appears %d times, want 1", libsqlTokenCount)
	}

	if libsqlShadowCount != 1 {
		t.Errorf("LIBSQL_SHADOW_DB_PATH appears %d times, want 1", libsqlShadowCount)
	}
}

func TestCreateOrUpdateEnvExampleSQLite(t *testing.T) {
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

	// Create .env.example with SQLite environment
	envs := []EnvironmentInput{
		{
			Name:         "development",
			DatabaseType: "sqlite",
		},
	}
	if err := createOrUpdateEnvExample(envs); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was created with SQLite examples
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "SQLITE_DB_PATH=./") {
		t.Error(".env.example should contain SQLITE_DB_PATH as a file path")
	}

	if !strings.Contains(contentStr, "SQLITE_SHADOW_DB_PATH=./") {
		t.Error(".env.example should contain SQLITE_SHADOW_DB_PATH as a file path")
	}

	// Should NOT contain libSQL or PostgreSQL variables for sqlite environment
	if strings.Contains(contentStr, "LIBSQL_") {
		t.Error(".env.example should not contain LIBSQL variables for sqlite environment")
	}
	if strings.Contains(contentStr, "POSTGRES_") {
		t.Error(".env.example should not contain POSTGRES variables for sqlite environment")
	}
}

func TestCreateOrUpdateEnvExampleLibSQL(t *testing.T) {
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

	// Create .env.example with libSQL environment
	envs := []EnvironmentInput{
		{
			Name:         "production",
			DatabaseType: "libsql",
		},
	}
	if err := createOrUpdateEnvExample(envs); err != nil {
		t.Fatalf("createOrUpdateEnvExample() error = %v", err)
	}

	// Verify file was created with libSQL examples
	content, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "LIBSQL_URL=libsql://") {
		t.Error(".env.example should contain LIBSQL_URL")
	}

	if !strings.Contains(contentStr, "LIBSQL_SHADOW_DB_PATH=./") {
		t.Error(".env.example should contain LIBSQL_SHADOW_DB_PATH as a file path (uses local shadow)")
	}

	// Should contain LIBSQL_AUTH_TOKEN for libsql environment
	if !strings.Contains(contentStr, "LIBSQL_AUTH_TOKEN=") {
		t.Error(".env.example should contain LIBSQL_AUTH_TOKEN for libsql environment")
	}

	// Should NOT contain PostgreSQL or SQLite variables for libsql environment
	if strings.Contains(contentStr, "POSTGRES_") {
		t.Error(".env.example should not contain POSTGRES variables for libsql environment")
	}
	if strings.Contains(contentStr, "SQLITE_") {
		t.Error(".env.example should not contain SQLITE variables for libsql environment")
	}
}

func TestGenerateLibSQLEnvironment(t *testing.T) {
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

	// Create libSQL environment
	environments := []EnvironmentInput{
		{
			Name:         "production",
			DatabaseType: "libsql",
			URL:          "libsql://mydb-myorg.turso.io",
			AuthToken:    "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.test.token",
		},
	}

	result, err := GenerateFiles(environments)
	if err != nil {
		t.Fatalf("GenerateFiles() error = %v", err)
	}

	// Verify .env.production was created
	if _, err := os.Stat(".env.production"); os.IsNotExist(err) {
		t.Error(".env.production was not created")
	}

	// Verify .env.production content
	envContent, err := os.ReadFile(".env.production")
	if err != nil {
		t.Fatalf("failed to read .env.production: %v", err)
	}

	envStr := string(envContent)
	if !strings.Contains(envStr, "LIBSQL_URL=libsql://mydb-myorg.turso.io?authToken=") {
		t.Error(".env.production should contain LIBSQL_URL with authToken")
	}

	if !strings.Contains(envStr, "LIBSQL_AUTH_TOKEN=eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.test.token") {
		t.Error(".env.production should contain LIBSQL_AUTH_TOKEN")
	}

	if !strings.Contains(envStr, "LIBSQL_SHADOW_DB_PATH=./schema/turso_shadow.db") {
		t.Error(".env.production should contain LIBSQL_SHADOW_DB_PATH as a file path")
	}

	// Verify .env.example was created and contains libSQL variables
	if !result.EnvExampleCreated {
		t.Error("expected .env.example to be created")
	}

	exampleContent, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	exampleStr := string(exampleContent)
	if !strings.Contains(exampleStr, "LIBSQL_URL=") {
		t.Error(".env.example should contain LIBSQL_URL")
	}
	if !strings.Contains(exampleStr, "LIBSQL_AUTH_TOKEN=") {
		t.Error(".env.example should contain LIBSQL_AUTH_TOKEN")
	}
	if !strings.Contains(exampleStr, "LIBSQL_SHADOW_DB_PATH=") {
		t.Error(".env.example should contain LIBSQL_SHADOW_DB_PATH")
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
	configContent, err := os.ReadFile("lockplane.toml")
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
