package cmd

import (
	"os"

	"github.com/mattsolo1/grove-core/cli"
	"github.com/mattsolo1/grove-docgen/pkg/generator"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate documentation for the current package",
		Long:  `Reads the docs/docgen.config.yml in the current directory, builds context, calls an LLM for each section, and writes the output to docs/dist/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			gen := generator.New(logger)

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			return gen.Generate(cwd)
		},
	}
	return cmd
}