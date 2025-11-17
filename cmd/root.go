// Package cmd implements the CLI commands for lockplane.
//
// This package follows the Cobra command pattern, with each command
// in its own file and a root command that ties them together.
package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var version = "0.9.0"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lockplane",
	Short: "Postgres-first database schema management",
	Long: `Lockplane is a schema management tool for PostgreSQL, SQLite, and libSQL.

It provides:
  • Schema introspection and diffing
  • Migration plan generation with validation
  • Shadow DB testing before production deployment
  • Automatic rollback generation
  • SQL validation and safety checks`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Set version from build info if available
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
		if version == "(devel)" {
			version = "dev"
		}

		// Extract commit and build time from vcs info
		var commit, buildTime string
		var modified bool

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				commit = setting.Value
				if len(commit) > 7 {
					commit = commit[:7] // Use short hash
				}
			case "vcs.modified":
				modified = setting.Value == "true"
			case "vcs.time":
				buildTime = setting.Value
			}
		}

		if commit != "" {
			version += fmt.Sprintf(" (%s", commit)
			if modified {
				version += " modified"
			}
			version += ")"
		}

		if buildTime != "" {
			version += fmt.Sprintf(" built %s", buildTime)
		}
	}

	rootCmd.Version = version
}
