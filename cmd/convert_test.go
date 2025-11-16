package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestConvertCommand(t *testing.T) {
	cmd := rootCmd
	var convertCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "convert" {
			convertCmd = c
			break
		}
	}

	if convertCmd == nil {
		t.Fatal("convert command should be registered")
	}

	if convertCmd.Short == "" {
		t.Error("convertCmd.Short should not be empty")
	}
}
