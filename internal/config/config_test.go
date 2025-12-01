package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const exampleConfig = `[environments.local]
postgres_url = "test"`

// compareConfigPaths compares two paths, resolving symlinks
func compareConfigPaths(t *testing.T, expected, actual string) {
	t.Helper()

	// Resolve symlinks for both paths
	expectedResolved, err := filepath.EvalSymlinks(expected)
	if err != nil {
		expectedResolved = expected
	}
	actualResolved, err := filepath.EvalSymlinks(actual)
	if err != nil {
		actualResolved = actual
	}

	if expectedResolved != actualResolved {
		t.Errorf("Expected ConfigFilePath=%q, got %q", expectedResolved, actualResolved)
	}
}

// changeToDir changes to a directory and returns a cleanup function
func changeToDir(t *testing.T, dir string) func() {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to change to directory %q: %v", dir, err)
	}

	return func() {
		// Check if original directory still exists before trying to return to it
		if _, err := os.Stat(originalDir); err == nil {
			if err := os.Chdir(originalDir); err != nil {
				t.Logf("Failed to restore working directory: %v", err)
			}
		}
	}
}

func TestLoadConfigInCurrentDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "lockplane.toml")
	configContent := exampleConfig

	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cleanup := changeToDir(t, tempDir)
	defer cleanup()

	config, err := LoadConfig()
	if err != nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if local, ok := config.Environments["local"]; ok {
		if local.PostgresURL != "test" {
			t.Errorf("Expected postgres_url=test, got %q", local.PostgresURL)
		}
	} else {
		t.Errorf("Expected local environment, got %q", local)
	}

	compareConfigPaths(t, configPath, config.ConfigFilePath)
}

func TestLoadConfigInParentDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "lockplane.toml")
	configContent := exampleConfig

	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create subdirectory and change to it
	subDir := filepath.Join(tempDir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	cleanup := changeToDir(t, subDir)
	defer cleanup()

	config, err := LoadConfig()
	if err != nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if local, ok := config.Environments["local"]; ok {
		if local.PostgresURL != "test" {
			t.Errorf("Expected postgres_url=test, got %q", local.PostgresURL)
		}
	} else {
		t.Errorf("Expected local environment, got %q", config.Environments)
	}

	compareConfigPaths(t, configPath, config.ConfigFilePath)
}

func TestLoadConfigNoFileReturnsEmpty(t *testing.T) {
	tempDir := t.TempDir()

	cleanup := changeToDir(t, tempDir)
	defer cleanup()

	config, err := LoadConfig()
	if err != nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if config.Environments != nil {
		t.Errorf("Expected empty environments, got %q", config.Environments)
	}

	if config.ConfigFilePath != "" {
		t.Errorf("Expected empty ConfigFilePath, got %q", config.ConfigFilePath)
	}
}

func TestLoadConfigStopsAtGitRoot(t *testing.T) {
	tempDir := t.TempDir()
	parentConfig := `[environments.local]
postgres_url = "parent"`
	gitProjectConfig := `[environments.local]
postgres_url = "git-project"`

	// Create a parent directory with lockplane.toml
	parentDir := filepath.Join(tempDir, "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatalf("Failed to create parent directory: %v", err)
	}
	parentConfigPath := filepath.Join(parentDir, "lockplane.toml")
	if err := os.WriteFile(parentConfigPath, []byte(parentConfig), 0o600); err != nil {
		t.Fatalf("Failed to write parent config: %v", err)
	}

	// Create a git project subdirectory with its own config
	gitProjectDir := filepath.Join(parentDir, "git-project")
	if err := os.MkdirAll(gitProjectDir, 0o755); err != nil {
		t.Fatalf("Failed to create git project directory: %v", err)
	}
	gitDir := filepath.Join(gitProjectDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}
	gitConfigPath := filepath.Join(gitProjectDir, "lockplane.toml")
	if err := os.WriteFile(gitConfigPath, []byte(gitProjectConfig), 0o600); err != nil {
		t.Fatalf("Failed to write git project config: %v", err)
	}

	// Create a subdirectory within git project
	subDir := filepath.Join(gitProjectDir, "src", "components")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	cleanup := changeToDir(t, subDir)
	defer cleanup()

	config, err := LoadConfig()
	if err != nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	// Should find the git-project config, not the parent config
	if local, ok := config.Environments["local"]; ok {
		if local.PostgresURL != "git-project" {
			t.Errorf("Expected postgres_url=git-project, got %q", local.PostgresURL)
		}
	} else {
		t.Errorf("Expected local environment, got %q", config.Environments)
	}

	compareConfigPaths(t, gitConfigPath, config.ConfigFilePath)
}

func TestLoadConfigStopsAtGoModRoot(t *testing.T) {
	tempDir := t.TempDir()

	// Create a parent directory with lockplane.toml
	parentDir := filepath.Join(tempDir, "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatalf("Failed to create parent directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parentDir, "lockplane.toml"), []byte(`default_environment = "parent"`), 0o600); err != nil {
		t.Fatalf("Failed to write parent config: %v", err)
	}

	// Create a Go module subdirectory (no config, should return empty)
	goModDir := filepath.Join(parentDir, "go-module")
	if err := os.MkdirAll(goModDir, 0o755); err != nil {
		t.Fatalf("Failed to create go module directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goModDir, "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create a subdirectory within go module
	subDir := filepath.Join(goModDir, "internal", "config")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	cleanup := changeToDir(t, subDir)
	defer cleanup()

	config, err := LoadConfig()
	if err != nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	// Should stop at go.mod boundary and return empty config
	if config.Environments != nil {
		t.Errorf("Expected empty environments, got %q", config.Environments)
	}

	if config.ConfigFilePath != "" {
		t.Errorf("Expected empty ConfigFilePath, got %q", config.ConfigFilePath)
	}
}

func TestLoadConfigStopsAtPackageJsonRoot(t *testing.T) {
	tempDir := t.TempDir()

	// Create a Node.js project directory
	nodeProjectDir := filepath.Join(tempDir, "node-project")
	if err := os.MkdirAll(nodeProjectDir, 0o755); err != nil {
		t.Fatalf("Failed to create node project directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeProjectDir, "package.json"), []byte(`{"name": "test"}`), 0o600); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(nodeProjectDir, "src")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	cleanup := changeToDir(t, subDir)
	defer cleanup()

	config, err := LoadConfig()
	if err != nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	// Should stop at package.json boundary and return empty config

	if config.Environments != nil {
		t.Errorf("Expected empty environments, got %q", config.Environments)
	}
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "lockplane.toml")
	invalidContent := `test = "test" invalid syntax`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0o600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cleanup := changeToDir(t, tempDir)
	defer cleanup()

	_, err := LoadConfig()
	if err == nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatal("Expected error for invalid TOML, got nil")
	}
	if !strings.Contains(err.Error(), "toml") {
		t.Errorf("Expected TOML parse error, got: %v", err)
	}
}

func TestLoadConfigEmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "lockplane.toml")

	if err := os.WriteFile(configPath, []byte(""), 0o600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cleanup := changeToDir(t, tempDir)
	defer cleanup()

	config, err := LoadConfig()
	if err != nil {
		PrintLoadConfigErrorDetails(err, t)
		t.Fatalf("LoadConfig returned error for empty file: %v", err)
	}

	if config.Environments != nil {
		t.Errorf("Expected empty environments, got %q", config.Environments)
	}

	compareConfigPaths(t, configPath, config.ConfigFilePath)
}

func TestIsProjectRootGit(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	if !isProjectRoot(tempDir) {
		t.Error("Expected isProjectRoot to return true for directory with .git")
	}
}

func TestIsProjectRootGoMod(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test\n"), 0o600); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	if !isProjectRoot(tempDir) {
		t.Error("Expected isProjectRoot to return true for directory with go.mod")
	}
}

func TestIsProjectRootPackageJson(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	packageJsonPath := filepath.Join(tempDir, "package.json")
	if err := os.WriteFile(packageJsonPath, []byte(`{"name": "test"}`), 0o600); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	if !isProjectRoot(tempDir) {
		t.Error("Expected isProjectRoot to return true for directory with package.json")
	}
}

func TestIsProjectRootNoMarkers(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	if isProjectRoot(tempDir) {
		t.Error("Expected isProjectRoot to return false for directory without project markers")
	}
}
