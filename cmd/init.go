package cmd

import (
	"os"

	initcmd "github.com/lockplane/lockplane/internal/commands/init"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new lockplane project",
	Long: `Initialize a new lockplane project with interactive prompts.

This will guide you through setting up:
  • Database connection configuration
  • Environment setup (local, staging, production)
  • lockplane.toml configuration file
  • .env files for each environment`,
	Run: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) {
	// Call the existing init implementation
	initcmd.RunInit(args)
	os.Exit(0) // RunInit may call log.Fatal, so we exit cleanly here
}
