package cmd

import (
	"fmt"
	"os"

	"github.com/lockplane/lockplane/internal/wizard"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new lockplane config",
	Long:  `Initialize a new lockplane config in the current directory`,
	Run:   runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool("force", false, "Overwrite existing lockplane.toml file")
}

func runInit(cmd *cobra.Command, args []string) {
	force, _ := cmd.Flags().GetBool("force")
	if err := wizard.Run(force); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
