package cmd

import (
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/database"
	"github.com/lockplane/lockplane/internal/executor"
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
	driver, err := executor.NewDriver("postgres")
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	// test db connection
	var postgresURL = cfg.Environments["local"].PostgresURL
	// TODO %s not allowed? what's %v?
	fmt.Printf("Testing connection to %v\n", postgresURL)
	driver.TestConnection(database.ConnectionConfig{
		PostgresUrl: postgresURL,
	})
	fmt.Println("Test successful")

	// introspect
	fmt.Println("Introspecting")
}
