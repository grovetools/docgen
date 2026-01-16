package cmd

import (
	"fmt"
	"os"

	"github.com/grovetools/docgen/pkg/config"
	"github.com/grovetools/docgen/pkg/parser"
	"github.com/spf13/cobra"
)

func newRegenJSONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "regen-json",
		Short: "Regenerate the structured JSON output from existing markdown files",
		Long: `Reads the docs/docgen.config.yml in the current directory, parses the existing generated markdown files, and regenerates the structured JSON output file.

This command does not call any LLMs or modify the markdown files. It's a quick way to update the JSON output if the parsing logic changes or if you have manually edited the markdown files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			cfg, err := config.Load(cwd)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no docgen.config.yml found in the current package. Run 'docgen init' first")
				}
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.Settings.StructuredOutputFile == "" {
				ulog.Info("No 'structured_output_file' configured. Nothing to do.").Emit()
				return nil
			}

			p := parser.New(getLogger())
			return p.GenerateJSON(cwd, cfg)
		},
	}
	return cmd
}