package cmd

import (
	"fmt"

	"github.com/mattsolo1/grove-core/cli"
	"github.com/mattsolo1/grove-docgen/internal/scaffold"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var projectType string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize docgen configuration and prompts for a package",
		Long: `Creates a default docs/docgen.config.yml and a set of starter prompt files in docs/prompts/ based on the specified project type.

This command provides a starting point for your documentation generation. It copies templates into your project, which you fully own and can modify as needed.

It will not overwrite existing files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			// For now, only 'library' is a valid type. This can be expanded later.
			if projectType != "library" {
				return fmt.Errorf("invalid project type '%s'. Currently, only 'library' is supported", projectType)
			}
			return scaffold.Init(projectType, logger)
		},
	}

	cmd.Flags().StringVar(&projectType, "type", "library", "Type of project to initialize (e.g., library)")

	return cmd
}