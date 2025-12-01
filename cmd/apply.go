package cmd

import (
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/database"
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

	// test db connection
	var PostgresURL = cfg.Environments["local"].PostgresURL
	// TODO %s not allowed? what's %v?
	fmt.Printf("Testing connection to %v\n", PostgresURL)
	database.TestConnection(database.ConnectionConfig{
		DatabaseType: "postgres",
		PostgresUrl:  PostgresURL,
	})
	fmt.Println("Test successful")
}
