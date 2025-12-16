package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
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
		printConfigNotFound()
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
	fmt.Println("loading schema")
	loadedSchema, err := schema.LoadSchema(dir)
	if err != nil {
		log.Fatalf("Failed to load schema: %v", err)
	}
	// loadedJsonBytes, err := json.MarshalIndent(loadedSchema, "", "  ")
	// if err != nil {
	// 	log.Fatalf("Failed to marshal schema to JSON: %v", err)
	// }
	// fmt.Println(string(loadedJsonBytes))
	fmt.Printf("Found %v tables\n", len(loadedSchema.Tables))

	// diff
	diff := schema.DiffSchemas(introspectedSchema, loadedSchema)

	// Check if there are any changes
	if diff.IsEmpty() {
		_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "\n✓ No changes detected - database already matches desired schema\n")
		os.Exit(0)
	}

	diffJsonBytes, err := json.MarshalIndent(diff, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal schema to JSON: %v", err)
	}
	fmt.Println(string(diffJsonBytes))
	fmt.Printf("Found %v added tables, %v modified tables, %v removed tables\n", len(diff.AddedTables), len(diff.ModifiedTables), len(diff.RemovedTables))

	// generate sql
	fmt.Println("Generating migration")
	sql := driver.GenerateMigration(diff)
	fmt.Println("Migration generated:")
	fmt.Printf("\n--\n\n%v\n\n--\n\n", sql)

	// apply
	fmt.Println("Applying migration")
	err = driver.ApplyMigration(ctx, db, sql)
	if err != nil {
		log.Fatalf("Failed to apply migration: %v", err)
	}
	fmt.Println("Migration applied successfully!")
}
