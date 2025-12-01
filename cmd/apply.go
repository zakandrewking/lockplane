package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/driver"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(applyCmd)
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a schema change to the database",
	Run:   runApply,
}

func runApply(cmd *cobra.Command, args []string) {
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
	fmt.Printf("Opening connection to %v\n", postgresURL)
	db, err := driver.OpenConnection(database.ConnectionConfig{
		PostgresUrl: postgresURL,
	})
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer func() { _ = db.Close() }()
	fmt.Println("Connection successful")

	// introspect
	fmt.Println("Introspecting")
	ctx := context.Background()
	introspectedSchema, err := driver.IntrospectSchema(ctx, db, "public")
	if err != nil {
		log.Fatalf("Failed to introspect schema: %v", err)
	}
	// jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	// if err != nil {
	// 	log.Fatalf("Failed to marshal schema to JSON: %v", err)
	// }
	// fmt.Println(string(jsonBytes))
	fmt.Printf("Found %v tables\n", len(introspectedSchema.Tables))

	// get scheme dir
	dir, err := config.GetSchemaDir()
	if err != nil {
		log.Fatalf("Failed to get schema directory: %v", err)
	}
	// load schema files
	loadedSchema, err := schema.LoadSchema(dir)
	if err != nil {
		log.Fatalf("Failed to load schema: %v", err)
	}
	loadedJsonBytes, err := json.MarshalIndent(loadedSchema, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal schema to JSON: %v", err)
	}
	fmt.Println(string(loadedJsonBytes))
}
