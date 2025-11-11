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

	expected := "sqlite://schema/test.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteConnectionStringDefault(t *testing.T) {
	env := EnvironmentInput{}

	connStr := BuildSQLiteConnectionString(env)

	expected := "sqlite://schema/lockplane.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteConnectionString() with defaults = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteShadowConnectionString(t *testing.T) {
	env := EnvironmentInput{
		FilePath: "schema/test.db",
	}

	connStr := BuildSQLiteShadowConnectionString(env)

	expected := "sqlite://schema/test_shadow.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteShadowConnectionString() = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteShadowConnectionStringDefault(t *testing.T) {
	env := EnvironmentInput{}

	connStr := BuildSQLiteShadowConnectionString(env)

	expected := "sqlite://schema/lockplane_shadow.db"
	if connStr != expected {
		t.Errorf("BuildSQLiteShadowConnectionString() with defaults = %q, want %q", connStr, expected)
	}
}

func TestBuildSQLiteShadowConnectionStringNoExtension(t *testing.T) {
	env := EnvironmentInput{
		FilePath: "data/mydb",
	}

	connStr := BuildSQLiteShadowConnectionString(env)

	expected := "sqlite://data/mydb_shadow"
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

	// For libSQL/Turso, shadow DB uses local SQLite
	expected := "sqlite://schema/turso_shadow.db"
	if connStr != expected {
		t.Errorf("BuildLibSQLShadowConnectionString() = %q, want %q", connStr, expected)
	}
}
