package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newSchemaGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate JSON schemas from Go source code",
		Long: `Executes 'go generate ./...' in the current directory.

This command provides a standardized way to trigger schema generation.
It relies on 'go:generate' directives within the Go source code to execute the actual schema generation tools.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			prettyLog.InfoPretty("Running 'go generate ./...' to create schemas...")

			execCmd := exec.Command("go", "generate", "./...")
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr

			if err := execCmd.Run(); err != nil {
				return fmt.Errorf("failed to run 'go generate': %w", err)
			}

			prettyLog.Success("✓ Schema generation complete.")
			return nil
		},
	}
	return cmd
}
