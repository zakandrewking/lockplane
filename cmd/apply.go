package cmd

import (
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/database/connection"
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
		log.Fatalf("Failed to load config: %v", err)
	}
	if cfg.ConfigFilePath == "" {
		fmt.Println(`lockplane.toml not found. Create one that looks like:

[environments.local]
postgres_url = "postgresql://postgres:postgres@localhost:5432/postgres"`)
		return
	}

	// create database driver
	// TODO move TestConnection into the driver, since we may have different sql
	// connection approaches
	driver, err := database.NewDriver("postgres")
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	// open db connection
	var postgresURL = cfg.Environments["local"].PostgresURL
	// TODO %s not allowed? what's %v?
	fmt.Printf("Opening connection to %v\n", postgresURL)
	db, err := driver.OpenConnection(connection.ConnectionConfig{
		PostgresUrl: postgresURL,
	})
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer func() { _ = db.Close() }()
	fmt.Println("Connection successful")

	// introspect
	fmt.Println("Introspecting")
	// ctx := context.Background()
	// schema, err := driver.IntrospectSchema(ctx, db)
	// if err != nil {
	// 	log.Fatalf("Failed to introspect schema: %v", err)
	// }
	// jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	// if err != nil {
	// 	log.Fatalf("Failed to marshal schema to JSON: %v", err)
	// }
	// fmt.Println(string(jsonBytes))
}
