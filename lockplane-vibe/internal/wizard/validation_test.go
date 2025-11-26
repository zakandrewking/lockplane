package wizard

import (
	"testing"
)

func TestValidateEnvironmentName(t *testing.T) {
	tests := []struct {
		name    string
		envName string
		wantErr bool
	}{
		{"valid lowercase", "local", false},
		{"valid uppercase", "PROD", false},
		{"valid with underscore", "my_env", false},
		{"valid with hyphen", "my-env", false},
		{"valid alphanumeric", "env123", false},
		{"empty name", "", true},
		{"with spaces", "my env", true},
		{"with special chars", "my@env", true},
		{"with slash", "my/env", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvironmentName(tt.envName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnvironmentName(%q) error = %v, wantErr %v", tt.envName, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{"valid port", "5432", false},
		{"valid max port", "65535", false},
		{"valid min port", "1", false},
		{"empty port", "", true},
		{"non-numeric", "abc", true},
		{"zero", "0", true},
		{"too large", "65536", true},
		{"negative", "-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort(%q) error = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
		})
	}
}

func TestValidateConnectionString(t *testing.T) {
	tests := []struct {
		name    string
		connStr string
		dbType  string
		wantErr bool
	}{
		{
			"valid postgres",
			"postgres://user:pass@localhost:5432/db",
			"postgres",
			false,
		},
		{
			"valid postgresql",
			"postgresql://user:pass@localhost:5432/db",
			"postgres",
			false,
		},
		{
			"invalid postgres prefix",
			"mysql://user:pass@localhost:5432/db",
			"postgres",
			true,
		},
		{
			"valid sqlite with prefix",
			"sqlite://path/to/db.db",
			"sqlite",
			false,
		},
		{
			"valid sqlite file path",
			"./db.db",
			"sqlite",
			false,
		},
		{
			"valid libsql",
			"libsql://db.turso.io",
			"libsql",
			false,
		},
		{
			"invalid libsql prefix",
			"http://db.turso.io",
			"libsql",
			true,
		},
		{
			"empty connection string",
			"",
			"postgres",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConnectionString(tt.connStr, tt.dbType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConnectionString(%q, %q) error = %v, wantErr %v",
					tt.connStr, tt.dbType, err, tt.wantErr)
			}
		})
	}
}

func TestBuildPostgresConnectionString(t *testing.T) {
	env := EnvironmentInput{
		Host:     "localhost",
		Port:     "5432",
		Database: "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	connStr := BuildPostgresConnectionString(env)

	expected := "postgresql://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	if connStr != expected {
		t.Errorf("BuildPostgresConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildPostgresConnectionStringRemote(t *testing.T) {
	env := EnvironmentInput{
		Host:     "db.example.com",
		Port:     "5432",
		Database: "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	connStr := BuildPostgresConnectionString(env)

	// Should use sslmode=require for remote hosts
	expected := "postgresql://testuser:testpass@db.example.com:5432/testdb?sslmode=require"
	if connStr != expected {
		t.Errorf("BuildPostgresConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildPostgresShadowConnectionString(t *testing.T) {
	env := EnvironmentInput{
		Host:     "localhost",
		Port:     "5432",
		Database: "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	connStr := BuildPostgresShadowConnectionString(env)

	expected := "postgresql://testuser:testpass@localhost:5433/testdb_shadow?sslmode=disable"
	if connStr != expected {
		t.Errorf("BuildPostgresShadowConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteConnectionString(t *testing.T) {
	env := EnvironmentInput{
		FilePath: "schema/test.db",
	}

	connStr := BuildSQLiteConnectionString(env)

	expected := "./schema/test.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteConnectionStringDefault(t *testing.T) {
	env := EnvironmentInput{}

	connStr := BuildSQLiteConnectionString(env)

	expected := "./schema/lockplane.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteConnectionString() with defaults = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteShadowConnectionString(t *testing.T) {
	env := EnvironmentInput{
		FilePath: "schema/test.db",
	}

	connStr := BuildSQLiteShadowConnectionString(env)

	expected := "./schema/test_shadow.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteShadowConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteShadowConnectionStringDefault(t *testing.T) {
	env := EnvironmentInput{}

	connStr := BuildSQLiteShadowConnectionString(env)

	expected := "./schema/lockplane_shadow.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteShadowConnectionString() with defaults = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteShadowConnectionStringNoExtension(t *testing.T) {
	env := EnvironmentInput{
		FilePath: "data/mydb",
	}

	connStr := BuildSQLiteShadowConnectionString(env)

	expected := "./data/mydb_shadow"
	if connStr != expected {
		t.Errorf("BuildSQLiteShadowConnectionString() with no extension = %q, want %q", connStr, expected)
	}
}

func TestBuildLibSQLConnectionString(t *testing.T) {
	env := EnvironmentInput{
		URL:       "libsql://db.turso.io",
		AuthToken: "token123",
	}

	connStr := BuildLibSQLConnectionString(env)

	expected := "libsql://db.turso.io?authToken=token123"
	if connStr != expected {
		t.Errorf("BuildLibSQLConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildLibSQLConnectionStringNoToken(t *testing.T) {
	env := EnvironmentInput{
		URL: "libsql://db.turso.io",
	}

	connStr := BuildLibSQLConnectionString(env)

	expected := "libsql://db.turso.io"
	if connStr != expected {
		t.Errorf("BuildLibSQLConnectionString() without token = %q, want %q", connStr, expected)
	}
}

func TestBuildLibSQLShadowConnectionString(t *testing.T) {
	env := EnvironmentInput{
		URL:       "libsql://db.turso.io",
		AuthToken: "token123",
	}

	connStr := BuildLibSQLShadowConnectionString(env)

	// For libSQL/Turso, shadow DB uses local SQLite file path
	expected := "./schema/turso_shadow.db"
	if connStr != expected {
		t.Errorf("BuildLibSQLShadowConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestParsePostgresConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		connStr  string
		wantEnv  EnvironmentInput
		wantErr  bool
		errMatch string
	}{
		{
			name:    "valid postgresql:// with all components",
			connStr: "postgresql://myuser:mypass@localhost:5432/mydb?sslmode=disable",
			wantEnv: EnvironmentInput{
				DatabaseType: "postgres",
				Host:         "localhost",
				Port:         "5432",
				Database:     "mydb",
				User:         "myuser",
				Password:     "mypass",
				SSLMode:      "disable",
				ShadowDBPort: "5433",
			},
			wantErr: false,
		},
		{
			name:    "valid postgres:// prefix",
			connStr: "postgres://testuser:testpass@db.example.com:5432/testdb?sslmode=require",
			wantEnv: EnvironmentInput{
				DatabaseType: "postgres",
				Host:         "db.example.com",
				Port:         "5432",
				Database:     "testdb",
				User:         "testuser",
				Password:     "testpass",
				SSLMode:      "require",
				ShadowDBPort: "5433",
			},
			wantErr: false,
		},
		{
			name:    "no port specified (uses default)",
			connStr: "postgresql://user:pass@localhost/mydb",
			wantEnv: EnvironmentInput{
				DatabaseType: "postgres",
				Host:         "localhost",
				Port:         "5432",
				Database:     "mydb",
				User:         "user",
				Password:     "pass",
				SSLMode:      "disable",
				ShadowDBPort: "5433",
			},
			wantErr: false,
		},
		{
			name:    "no sslmode (auto-detect for localhost)",
			connStr: "postgresql://user:pass@localhost:5432/mydb",
			wantEnv: EnvironmentInput{
				DatabaseType: "postgres",
				Host:         "localhost",
				Port:         "5432",
				Database:     "mydb",
				User:         "user",
				Password:     "pass",
				SSLMode:      "disable",
				ShadowDBPort: "5433",
			},
			wantErr: false,
		},
		{
			name:    "no sslmode (auto-detect for remote)",
			connStr: "postgresql://user:pass@db.example.com:5432/mydb",
			wantEnv: EnvironmentInput{
				DatabaseType: "postgres",
				Host:         "db.example.com",
				Port:         "5432",
				Database:     "mydb",
				User:         "user",
				Password:     "pass",
				SSLMode:      "require",
				ShadowDBPort: "5433",
			},
			wantErr: false,
		},
		{
			name:    "password with special characters",
			connStr: "postgresql://user:p@ss%3Aword@localhost:5432/mydb",
			wantEnv: EnvironmentInput{
				DatabaseType: "postgres",
				Host:         "localhost",
				Port:         "5432",
				Database:     "mydb",
				User:         "user",
				Password:     "p@ss:word",
				SSLMode:      "disable",
				ShadowDBPort: "5433",
			},
			wantErr: false,
		},
		{
			name:     "invalid prefix",
			connStr:  "mysql://user:pass@localhost:3306/mydb",
			wantErr:  true,
			errMatch: "must start with postgres:// or postgresql://",
		},
		{
			name:     "missing host",
			connStr:  "postgresql://user:pass@:5432/mydb",
			wantErr:  true,
			errMatch: "missing host",
		},
		{
			name:     "missing database",
			connStr:  "postgresql://user:pass@localhost:5432",
			wantErr:  true,
			errMatch: "missing database name",
		},
		{
			name:     "missing user",
			connStr:  "postgresql://:pass@localhost:5432/mydb",
			wantErr:  true,
			errMatch: "missing user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := ParsePostgresConnectionString(tt.connStr)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePostgresConnectionString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errMatch != "" && err != nil {
					if !containsString(err.Error(), tt.errMatch) {
						t.Errorf("ParsePostgresConnectionString() error = %q, want error containing %q", err.Error(), tt.errMatch)
					}
				}
				return
			}

			// Compare environments
			if env.DatabaseType != tt.wantEnv.DatabaseType {
				t.Errorf("DatabaseType = %q, want %q", env.DatabaseType, tt.wantEnv.DatabaseType)
			}
			if env.Host != tt.wantEnv.Host {
				t.Errorf("Host = %q, want %q", env.Host, tt.wantEnv.Host)
			}
			if env.Port != tt.wantEnv.Port {
				t.Errorf("Port = %q, want %q", env.Port, tt.wantEnv.Port)
			}
			if env.Database != tt.wantEnv.Database {
				t.Errorf("Database = %q, want %q", env.Database, tt.wantEnv.Database)
			}
			if env.User != tt.wantEnv.User {
				t.Errorf("User = %q, want %q", env.User, tt.wantEnv.User)
			}
			if env.Password != tt.wantEnv.Password {
				t.Errorf("Password = %q, want %q", env.Password, tt.wantEnv.Password)
			}
			if env.SSLMode != tt.wantEnv.SSLMode {
				t.Errorf("SSLMode = %q, want %q", env.SSLMode, tt.wantEnv.SSLMode)
			}
			if env.ShadowDBPort != tt.wantEnv.ShadowDBPort {
				t.Errorf("ShadowDBPort = %q, want %q", env.ShadowDBPort, tt.wantEnv.ShadowDBPort)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
