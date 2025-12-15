package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/driver"
)

// TestRunIntrospect_Success tests that introspect successfully introspects a database
func TestRunIntrospect_Success(t *testing.T) {
	// Skip integration tests when running with -short flag
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Get database URL from environment
	dbUrl := os.Getenv("POSTGRES_URL")
	if dbUrl == "" {
		t.Skip("Skipping test: POSTGRES_URL not set")
	}

	// Check if we're being run as a subprocess
	if os.Getenv("TEST_RUN_INTROSPECT_SUCCESS") == "1" {
		// This will run in the subprocess
		tmpDir := os.Getenv("TEST_TMPDIR")
		if tmpDir == "" {
			t.Fatal("TEST_TMPDIR not set")
		}

		// Change to temp directory
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change to temp directory: %v", err)
		}

		// Run introspect (config file was created by parent process)
		rootCmd.SetArgs([]string{"introspect"})
		_ = rootCmd.Execute()
		return
	}

	// Main test process
	tmpDir := t.TempDir()

	// Create test database connection to set up test table
	ctx := context.Background()
	driver, err := driver.NewDriver(database.DatabaseTypePostgres)
	if err != nil {
		t.Skipf("Skipping test: cannot create driver: %v", err)
	}

	db, err := driver.OpenConnection(database.ConnectionConfig{
		PostgresUrl: dbUrl,
	})
	if err != nil {
		t.Skipf("Skipping test: cannot open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	tableName := "test_introspect_cmd_success"
	_, err = db.ExecContext(ctx, `
		CREATE TABLE `+tableName+` (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE "+tableName) }()

	// Create lockplane.toml in temp directory
	configContent := `[environments.local]
postgres_url = "` + dbUrl + `"
`
	configPath := filepath.Join(tmpDir, "lockplane.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Run introspect command in subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestRunIntrospect_Success")
	cmd.Env = append(os.Environ(), "TEST_RUN_INTROSPECT_SUCCESS=1", "TEST_TMPDIR="+tmpDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("introspect command failed: %v\nOutput: %s", err, string(output))
	}

	// Verify output contains valid JSON
	outputStr := string(output)

	// Find JSON in output (skip the log messages)
	jsonStart := strings.Index(outputStr, "{")
	if jsonStart == -1 {
		t.Fatalf("Expected JSON output, got: %s", outputStr)
	}

	// Parse JSON to verify it's valid (use Decoder to handle extra text after JSON)
	var schema database.Schema
	decoder := json.NewDecoder(strings.NewReader(outputStr[jsonStart:]))
	if err := decoder.Decode(&schema); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, outputStr)
	}

	// Verify the test table is in the schema
	var testTable *database.Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == tableName {
			testTable = &schema.Tables[i]
			break
		}
	}

	if testTable == nil {
		t.Fatalf("Expected to find table %q in introspected schema", tableName)
	}

	// Verify table has expected columns
	if len(testTable.Columns) != 3 {
		t.Errorf("Expected 3 columns in test table, got %d", len(testTable.Columns))
	}

	// Helper to find column by name
	findColumn := func(columns []database.Column, name string) *database.Column {
		for i := range columns {
			if columns[i].Name == name {
				return &columns[i]
			}
		}
		return nil
	}

	// Check id column
	idCol := findColumn(testTable.Columns, "id")
	if idCol == nil {
		t.Fatal("Expected to find 'id' column")
	}
	if idCol.Type != "integer" {
		t.Errorf("Expected id type 'integer', got %q", idCol.Type)
	}
	if idCol.Nullable {
		t.Error("Expected id to be NOT NULL")
	}
	if !idCol.IsPrimaryKey {
		t.Error("Expected id to be PRIMARY KEY")
	}

	// Check name column
	nameCol := findColumn(testTable.Columns, "name")
	if nameCol == nil {
		t.Fatal("Expected to find 'name' column")
	}
	if nameCol.Type != "text" {
		t.Errorf("Expected name type 'text', got %q", nameCol.Type)
	}
	if nameCol.Nullable {
		t.Error("Expected name to be NOT NULL")
	}
	if nameCol.IsPrimaryKey {
		t.Error("Expected name to NOT be PRIMARY KEY")
	}

	// Check email column
	emailCol := findColumn(testTable.Columns, "email")
	if emailCol == nil {
		t.Fatal("Expected to find 'email' column")
	}
	if emailCol.Type != "text" {
		t.Errorf("Expected email type 'text', got %q", emailCol.Type)
	}
	if !emailCol.Nullable {
		t.Error("Expected email to be nullable")
	}
	if emailCol.IsPrimaryKey {
		t.Error("Expected email to NOT be PRIMARY KEY")
	}
}
