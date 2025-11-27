package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lockplane",
	Short: "Lockplane is a tool for managing PostgreSQL schema migrations.",
	Long:  `Lockplane is a tool for managing PostgreSQL schema migrations.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
