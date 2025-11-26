package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/shadow"
	"github.com/spf13/cobra"
)

var (
	shadowEnv          string
	shadowShadowDB     string
	shadowShadowSchema string
	shadowVerbose      bool
	shadowOutputJSON   bool
	shadowForcePrepare bool
	shadowDiffEnv      string
)

var shadowCmd = &cobra.Command{
	Use:   "shadow",
	Short: "Manage the Lockplane shadow database",
}

func init() {
	shadowPrepareCmd := &cobra.Command{
		Use:   "prepare",
		Short: "Reset the shadow database and return a connection URL",
		Run:   runShadowPrepare,
	}
	shadowPrepareCmd.Flags().StringVar(&shadowEnv, "target-environment", "", "Target environment to prepare")
	shadowPrepareCmd.Flags().StringVar(&shadowShadowDB, "shadow-db", "", "Override shadow database URL")
	shadowPrepareCmd.Flags().StringVar(&shadowShadowSchema, "shadow-schema", "", "Override shadow schema (Postgres only)")
	shadowPrepareCmd.Flags().BoolVar(&shadowVerbose, "verbose", false, "Enable verbose logging")
	shadowPrepareCmd.Flags().BoolVar(&shadowOutputJSON, "json", true, "Output JSON metadata")
	shadowPrepareCmd.Flags().BoolVar(&shadowForcePrepare, "force", false, "Overwrite existing reservation if present")

	shadowDiffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff the prepared shadow database against an environment",
		Run:   runShadowDiff,
	}
	shadowDiffCmd.Flags().StringVar(&shadowDiffEnv, "target-environment", "", "Environment to compare against (defaults to reservation)")

	shadowReleaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Release any active shadow reservation",
		Run:   runShadowRelease,
	}

	shadowCmd.AddCommand(shadowPrepareCmd, shadowDiffCmd, shadowReleaseCmd)
	rootCmd.AddCommand(shadowCmd)
}

func runShadowPrepare(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	env, err := config.ResolveEnvironment(cfg, shadowEnv)
	if err != nil {
		log.Fatalf("Failed to resolve environment: %v", err)
	}

	shadowURL := strings.TrimSpace(shadowShadowDB)
	shadowSchema := strings.TrimSpace(shadowShadowSchema)
	if shadowURL == "" {
		shadowURL = env.ShadowDatabaseURL
		if shadowURL == "" && env.ShadowSchema != "" {
			shadowURL = env.DatabaseURL
		}
	}
	if shadowSchema == "" {
		shadowSchema = env.ShadowSchema
	}

	if shadowURL == "" {
		fmt.Fprintf(os.Stderr, "Error: No shadow database configured.\n\n")
		fmt.Fprintf(os.Stderr, "Set shadow_database_url in lockplane.toml or pass --shadow-db.\n")
		fmt.Fprintf(os.Stderr, "Use 'lockplane init' to scaffold shadow DB settings.\n")
		os.Exit(1)
	}

	if !shadowForcePrepare {
		if existing, err := shadow.LoadReservation(); err == nil && existing != nil {
			fmt.Fprintf(os.Stderr, "Shadow DB already prepared for environment %q.\nUse --force or 'lockplane shadow release' to overwrite.\n", existing.Environment)
			os.Exit(1)
		}
	}

	driverType := executor.DetectDriver(shadowURL)
	sqlDriverName := executor.GetSQLDriverName(driverType)
	db, err := sql.Open(sqlDriverName, shadowURL)
	if err != nil {
		log.Fatalf("Failed to open shadow database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping shadow database: %v", err)
	}

	driver, err := executor.NewDriver(driverType)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	if shadowSchema != "" && driver.SupportsSchemas() {
		if err := driver.CreateSchema(ctx, db, shadowSchema); err != nil {
			log.Fatalf("Failed to create shadow schema: %v", err)
		}
		if err := driver.SetSchema(ctx, db, shadowSchema); err != nil {
			log.Fatalf("Failed to set shadow schema: %v", err)
		}
	}

	if err := executor.CleanupShadowDB(ctx, db, driver, shadowVerbose); err != nil {
		log.Fatalf("Failed to clean shadow database: %v", err)
	}

	reservation := &shadow.Reservation{
		Environment:  env.Name,
		ShadowURL:    shadowURL,
		ShadowSchema: shadowSchema,
		CreatedAt:    time.Now().UTC(),
	}
	if err := shadow.SaveReservation(reservation); err != nil {
		log.Fatalf("Failed to save reservation: %v", err)
	}

	if shadowOutputJSON {
		output, _ := json.MarshalIndent(reservation, "", "  ")
		fmt.Println(string(output))
	} else {
		fmt.Fprintf(os.Stderr, "Shadow DB prepared for %s\nConnection: %s\n", env.Name, shadowURL)
		if shadowSchema != "" {
			fmt.Fprintf(os.Stderr, "Schema: %s\n", shadowSchema)
		}
	}
}

func runShadowDiff(cmd *cobra.Command, args []string) {
	reservation, err := shadow.LoadReservation()
	if err != nil {
		log.Fatalf("Failed to load reservation: %v", err)
	}
	if reservation == nil {
		fmt.Fprintf(os.Stderr, "No active shadow reservation. Run 'lockplane shadow prepare' first.\n")
		os.Exit(1)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	targetEnv := shadowDiffEnv
	if targetEnv == "" {
		targetEnv = reservation.Environment
	}

	resolvedEnv, err := config.ResolveEnvironment(cfg, targetEnv)
	if err != nil {
		log.Fatalf("Failed to resolve environment: %v", err)
	}

	fromConn := strings.TrimSpace(resolvedEnv.DatabaseURL)
	if fromConn == "" {
		fmt.Fprintf(os.Stderr, "Environment %q does not define a database_url.\n", resolvedEnv.Name)
		os.Exit(1)
	}

	before, err := executor.LoadSchemaFromConnectionString(fromConn)
	if err != nil {
		log.Fatalf("Failed to introspect %s: %v", resolvedEnv.Name, err)
	}

	after, err := executor.LoadSchemaFromConnectionString(reservation.ShadowURL)
	if err != nil {
		log.Fatalf("Failed to load shadow schema: %v", err)
	}

	diff := schema.DiffSchemas(before, after)
	if diff.IsEmpty() {
		_, _ = color.New(color.FgGreen).Fprintf(os.Stderr, "✓ No differences between %s and prepared shadow\n", resolvedEnv.Name)
		return
	}

	driverType := executor.DetectDriver(fromConn)
	driver, err := executor.NewDriver(driverType)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	plan, err := planner.GeneratePlanWithHash(diff, before, driver)
	if err != nil {
		log.Fatalf("Failed to generate plan: %v", err)
	}

	output, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal plan: %v", err)
	}
	fmt.Println(string(output))
}

func runShadowRelease(cmd *cobra.Command, args []string) {
	if err := shadow.ClearReservation(); err != nil {
		log.Fatalf("Failed to release reservation: %v", err)
	}
	fmt.Fprintln(os.Stderr, "Shadow reservation cleared.")
}
