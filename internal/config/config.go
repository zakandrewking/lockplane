package config

import (
	"os"
	"path/filepath"

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

func LoadConfig() (*Config, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dir := startDir
	for {
		// Check if lockplane.toml exists in current directory
		configPath := filepath.Join(dir, "lockplane.toml")
		if _, err := os.Stat(configPath); err == nil {
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

	return &Config{}, nil
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
