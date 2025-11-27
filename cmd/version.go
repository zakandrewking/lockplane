package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the Lockplane version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getVersion())
	},
}

func getVersion() string {
	version := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
		if version == "(devel)" {
			version = "dev"
		}

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
	return version
}
