package cmd

import (
	"os"

	"github.com/mattsolo1/grove-core/logging"
	"github.com/mattsolo1/grove-docgen/pkg/generator"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var sections []string
	
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate documentation for the current package",
		Long: `Reads the docs/docgen.config.yml in the current directory, builds context, calls an LLM for each section, and writes the output to docs/.

Examples:
  docgen generate                          # Generate all sections
  docgen generate --section introduction   # Generate only introduction
  docgen generate -s intro -s core         # Generate multiple specific sections`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.NewLogger("grove-docgen")
			gen := generator.New(logger.Logger)

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			// If sections are specified, use GenerateWithOptions
			if len(sections) > 0 {
				opts := generator.GenerateOptions{
					Sections: sections,
				}
				return gen.GenerateWithOptions(cwd, opts)
			}
			
			// Otherwise generate all sections
			return gen.Generate(cwd)
		},
	}
	
	cmd.Flags().StringSliceVarP(&sections, "section", "s", nil, "Generate only specified sections (by name)")
	
	return cmd
}