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
	initCmd.Flags().Bool("yes", false, "Skip interactive prompt and proceed immediately")
}

func runInit(cmd *cobra.Command, args []string) {
	force, _ := cmd.Flags().GetBool("force")
	yes, _ := cmd.Flags().GetBool("yes")
	if err := wizard.Run(force, yes); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
