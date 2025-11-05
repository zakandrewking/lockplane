package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultEnvironmentName   = "local"
	defaultDatabaseURL       = "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
	defaultShadowDatabaseURL = "postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable"
	defaultSchemaDir         = "schema"
)

// EnvironmentConfig describes a single named environment from lockplane.toml.
type EnvironmentConfig struct {
	Description       string `toml:"description"`
	DatabaseURL       string `toml:"database_url"`
	ShadowDatabaseURL string `toml:"shadow_database_url"`
	SchemaPath        string `toml:"schema_path"`
}

// Config represents the lockplane.toml configuration file.
type Config struct {
	DefaultEnvironment string                       `toml:"default_environment"`
	SchemaPath         string                       `toml:"schema_path"`
	DatabaseURL        string                       `toml:"database_url"`        // legacy fallback
	ShadowDatabaseURL  string                       `toml:"shadow_database_url"` // legacy fallback
	Environments       map[string]EnvironmentConfig `toml:"environments"`
	configDir          string                       `toml:"-"`
	projectDir         string                       `toml:"-"`
	configFilePath     string                       `toml:"-"`
}

// LoadConfig loads the lockplane.toml file from the current directory or any parent directory.
func LoadConfig() (*Config, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dir := startDir
	for {
		candidates := []string{
			filepath.Join(dir, "lockplane.toml"),
			filepath.Join(dir, defaultSchemaDir, "lockplane.toml"),
		}

		for _, configPath := range candidates {
			if _, err := os.Stat(configPath); err == nil {
				data, err := os.ReadFile(configPath)
				if err != nil {
					return nil, err
				}

				var config Config
				if err := toml.Unmarshal(data, &config); err != nil {
					return nil, err
				}

				config.configFilePath = configPath
				config.configDir = filepath.Dir(configPath)
				config.projectDir = dir
				return &config, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return &Config{configDir: startDir, projectDir: startDir}, nil
}

// ConfigDir returns the directory containing the resolved lockplane.toml (or the working directory if none exists).
func (c *Config) ConfigDir() string {
	if c == nil {
		return ""
	}
	if c.configDir != "" {
		return c.configDir
	}
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return ""
}

// ProjectDir returns the project root associated with the loaded configuration.
func (c *Config) ProjectDir() string {
	if c == nil {
		return ""
	}
	if c.projectDir != "" {
		return c.projectDir
	}
	return c.ConfigDir()
}

// ConfigFile returns the absolute path to the resolved lockplane.toml, if any.
func (c *Config) ConfigFile() string {
	if c == nil {
		return ""
	}
	return c.configFilePath
}

// GetSchemaPath returns the schema path with priority: explicit value > environment config > global config > default.
func GetSchemaPath(explicitValue string, config *Config, env *ResolvedEnvironment, defaultValue string) string {
	if explicitValue != "" {
		return explicitValue
	}
	if env != nil && env.SchemaPath != "" {
		return resolveSchemaPath(env.SchemaPath, env.ResolvedConfigDir)
	}
	if config != nil && config.SchemaPath != "" {
		return resolveSchemaPath(config.SchemaPath, config.ConfigDir())
	}
	if defaultValue != "" {
		base := ""
		if env != nil && env.ResolvedConfigDir != "" {
			base = env.ResolvedConfigDir
		} else if config != nil {
			base = config.ConfigDir()
		}
		return resolveSchemaPath(defaultValue, base)
	}
	return defaultValue
}

func resolveSchemaPath(value, base string) string {
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) || base == "" {
		return value
	}
	return filepath.Join(base, value)
}
