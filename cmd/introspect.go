package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/driver"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(introspectCmd)
}

var introspectCmd = &cobra.Command{
	Use:   "introspect",
	Short: "Introspect the database schema",
	Long:  "Introspect the database schema and generate a JSON representation of the schema",
	Run:   runIntrospect,
}

func runIntrospect(cmd *cobra.Command, args []string) {
	// load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		config.PrintLoadConfigErrorDetails(err, nil)
		log.Fatalf("Failed to load config: %v", err)
	}
	if cfg.ConfigFilePath == "" {
		fmt.Println(`lockplane.toml not found. Create one that looks like:

[environments.local]
postgres_url = "postgresql://postgres:postgres@localhost:5432/postgres"`)
		return
	}

	// create database driver
	driver, err := driver.NewDriver(database.DatabaseTypePostgres)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	// open db connection
	var postgresURL string
	if local, ok := cfg.Environments["local"]; !ok {
		log.Fatalf("Environment 'local' not found in config")
	} else {
		postgresURL = local.PostgresURL
	}
	fmt.Fprintf(os.Stderr, "Opening connection to %v\n", postgresURL)
	db, err := driver.OpenConnection(database.ConnectionConfig{
		PostgresUrl: postgresURL,
	})
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer func() { _ = db.Close() }()
	fmt.Fprintln(os.Stderr, "Connection successful")

	// introspect
	fmt.Fprintln(os.Stderr, "Introspecting")
	ctx := context.Background()
	introspectedSchema, err := driver.IntrospectSchema(ctx, db, "public")
	if err != nil {
		log.Fatalf("Failed to introspect schema: %v", err)
	}

	// output schema as JSON
	jsonBytes, err := json.MarshalIndent(introspectedSchema, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal schema to JSON: %v", err)
	}
	fmt.Println(string(jsonBytes))
}
