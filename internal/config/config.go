package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// EnvironmentConfig describes a single named environment from lockplane.toml.
type EnvironmentConfig struct {
	PostgresURL string `toml:"postgres_url"`
}

type Config struct {
	Environments   map[string]EnvironmentConfig `toml:"environments"`
	ConfigFilePath string                       `toml:"-"`
}

// Useful to provide better error details from LoadConfig
func PrintLoadConfigErrorDetails(err error, t *testing.T) {
	var derr *toml.DecodeError
	if errors.As(err, &derr) {
		if t != nil {
			t.Log(derr.String())
			row, col := derr.Position()
			t.Logf("Error occurred at row %d, column %d", row, col)
		} else {
			fmt.Println(derr.String())
			row, col := derr.Position()
			fmt.Printf("Error occurred at row %d, column %d\n", row, col)
		}
	}
}

func LoadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	config.ConfigFilePath = configPath
	return &config, nil
}

func getConfigPath() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := startDir
	for {
		// Check if lockplane.toml exists in current directory
		configPath := filepath.Join(dir, "lockplane.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Check if we've reached a project boundary
		if isProjectRoot(dir) {
			break
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("lockplane.toml not found")
}

// isProjectRoot checks if the directory is a project root based on common markers
func isProjectRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return true
	}
	return false
}

func GetSchemaDir() (string, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return "", err
	}
	configDir := filepath.Dir(configPath)
	schemaDir := filepath.Join(configDir, "schema")
	if info, err := os.Stat(schemaDir); err == nil && info.IsDir() {
		return schemaDir, nil
	}
	return "", fmt.Errorf("Schema directory not found. Try creating schema/ in the same directory as lockplane.toml.")
}
