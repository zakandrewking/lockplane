package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lockplane/lockplane/database/postgres"
	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/spf13/cobra"
)

var introspectCmd = &cobra.Command{
	Use:   "introspect",
	Short: "Introspect a database and output its schema",
	Long: `Introspect a database and output its schema in JSON or SQL DDL format.

The database can be specified via:
  1. --db flag (highest priority)
  2. --source-environment or default environment from lockplane.toml
  3. Built-in defaults (postgres on localhost)`,
	Example: `  # Introspect to JSON (default)
  lockplane introspect > schema.json

  # Introspect to SQL DDL
  lockplane introspect --format sql > lockplane/schema.lp.sql

  # Specify database connection directly
  lockplane introspect --db postgresql://localhost:5432/myapp?sslmode=disable > schema.json

  # Introspect Supabase local database to SQL
  lockplane introspect --db postgresql://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable --format sql > schema.lp.sql`,
	Run: runIntrospect,
}

var (
	introspectDB        string
	introspectFormat    string
	introspectSourceEnv string
	introspectUseShadow bool
	introspectVerbose   bool
)

func init() {
	rootCmd.AddCommand(introspectCmd)

	introspectCmd.Flags().StringVar(&introspectDB, "db", "", "Database connection string (overrides environment selection)")
	introspectCmd.Flags().StringVar(&introspectFormat, "format", "json", "Output format: json or sql")
	introspectCmd.Flags().StringVar(&introspectSourceEnv, "source-environment", "", "Named environment to introspect (defaults to config default)")
	introspectCmd.Flags().BoolVar(&introspectUseShadow, "shadow", false, "Use the shadow database URL for the selected environment")
	introspectCmd.Flags().BoolVarP(&introspectVerbose, "verbose", "v", false, "Enable verbose logging")
}

func runIntrospect(cmd *cobra.Command, args []string) {
	// Load config file (if it exists)
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	connStr := strings.TrimSpace(introspectDB)
	var resolvedEnv *config.ResolvedEnvironment
	if connStr == "" {
		// If no source-environment provided, try to use default environment
		envName := strings.TrimSpace(introspectSourceEnv)
		if envName == "" {
			envName = cfg.DefaultEnvironment
			if envName == "" {
				envName = "local"
			}
			if introspectVerbose {
				fmt.Fprintf(os.Stderr, "ℹ️  Using default environment: %s\n", envName)
			}
		}

		resolvedEnv, err = config.ResolveEnvironment(cfg, envName)
		if err != nil {
			log.Fatalf("Failed to resolve source environment: %v", err)
		}
		connStr = resolvedEnv.DatabaseURL
		if introspectUseShadow {
			connStr = resolvedEnv.ShadowDatabaseURL
			if connStr == "" {
				log.Fatalf("Environment %q does not define a shadow database URL", resolvedEnv.Name)
			}
		}
	}

	if connStr == "" {
		envName := "local"
		if resolvedEnv != nil {
			envName = resolvedEnv.Name
		} else if cfg != nil && cfg.DefaultEnvironment != "" {
			envName = cfg.DefaultEnvironment
		}
		log.Fatalf("No database connection configured. Provide --db or configure environment %q in lockplane.toml / .env.%s.", envName, envName)
	}

	if introspectVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Introspecting database: %s\n", connStr)
	}

	// Detect database driver from connection string
	driverType := executor.DetectDriver(connStr)
	driver, err := executor.NewDriver(driverType)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	// Get the SQL driver name (use detected type, not driver.Name())
	sqlDriverName := executor.GetSQLDriverName(driverType)

	db, err := sql.Open(sqlDriverName, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	schema, err := driver.IntrospectSchema(ctx, db)
	if err != nil {
		log.Fatalf("Failed to introspect schema: %v", err)
	}

	// Output in requested format
	switch introspectFormat {
	case "json":
		jsonBytes, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal schema to JSON: %v", err)
		}
		fmt.Println(string(jsonBytes))

	case "sql":
		// Generate SQL DDL from schema
		// Use PostgreSQL driver for SQL generation (works for most databases)
		sqlDriver := postgres.NewDriver()
		var sqlBuilder strings.Builder

		for _, table := range schema.Tables {
			sql, _ := sqlDriver.CreateTable(table)
			sqlBuilder.WriteString(sql)
			sqlBuilder.WriteString(";\n\n")

			// Add indexes
			for _, idx := range table.Indexes {
				sql, _ := sqlDriver.AddIndex(table.Name, idx)
				sqlBuilder.WriteString(sql)
				sqlBuilder.WriteString(";\n")
			}

			if len(table.Indexes) > 0 {
				sqlBuilder.WriteString("\n")
			}

			// Add foreign keys
			for _, fk := range table.ForeignKeys {
				sql, _ := sqlDriver.AddForeignKey(table.Name, fk)
				if !strings.HasPrefix(sql, "--") { // Skip comment-only SQL
					sqlBuilder.WriteString(sql)
					sqlBuilder.WriteString(";\n")
				}
			}

			if len(table.ForeignKeys) > 0 {
				sqlBuilder.WriteString("\n")
			}
		}

		fmt.Print(sqlBuilder.String())

	default:
		log.Fatalf("Unsupported format: %s (use 'json' or 'sql')", introspectFormat)
	}
}
