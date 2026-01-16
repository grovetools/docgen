package cmd

import (
	"fmt"

	"github.com/grovetools/docgen/internal/scaffold"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var projectType string
	var opts scaffold.InitOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize docgen configuration and prompts for a package",
		Long: `Creates a default docs/docgen.config.yml and a set of starter prompt files in docs/prompts/ based on the specified project type.

This command provides a starting point for your documentation generation. It copies templates into your project, which you fully own and can modify as needed.

It will not overwrite existing files.

Examples:
  docgen init                                    # Initialize with defaults
  docgen init --model gemini-2.0-flash-latest    # Use a specific model
  docgen init --rules-file custom.rules          # Use a custom rules file
  docgen init --output-dir generated-docs        # Output to a different directory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// For now, only 'library' is a valid type. This can be expanded later.
			if projectType != "library" {
				return fmt.Errorf("invalid project type '%s'. Currently, only 'library' is supported", projectType)
			}
			return scaffold.InitWithOptions(projectType, opts, getLogger())
		},
	}

	cmd.Flags().StringVar(&projectType, "type", "library", "Type of project to initialize (e.g., library)")
	cmd.Flags().StringVar(&opts.Model, "model", "", "LLM model to use for generation")
	cmd.Flags().StringVar(&opts.RegenerationMode, "regeneration-mode", "", "Regeneration mode: scratch or reference")
	cmd.Flags().StringVar(&opts.RulesFile, "rules-file", "", "Rules file for context generation")
	cmd.Flags().StringVar(&opts.StructuredOutputFile, "structured-output-file", "", "Path for structured JSON output")
	cmd.Flags().StringVar(&opts.SystemPrompt, "system-prompt", "", "System prompt: 'default' or path to custom prompt file")
	cmd.Flags().StringVar(&opts.OutputDir, "output-dir", "", "Output directory for generated documentation")

	return cmd
}