package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/mattsolo1/grove-docgen/pkg/generator"
	"github.com/mattsolo1/grove-docgen/pkg/readme"
	"github.com/spf13/cobra"
)

func newSyncReadmeCmd() *cobra.Command {
	var generateSource bool

	cmd := &cobra.Command{
		Use:   "sync-readme",
		Short: "Generate the README.md from a template and a source documentation file",
		Long: `Synchronizes the project's README.md based on the 'readme' configuration in docs/docgen.config.yml.

This command reads a template file (e.g., README.md.tpl), injects a specified documentation section (like 'introduction') into it, replaces metadata placeholders, and writes the result to the output README.md file.

It provides a single source of truth for your project's overview, keeping the README in sync with your formal documentation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			if generateSource {
				cfg, err := config.Load(cwd)
				if err != nil {
					return fmt.Errorf("could not load config to generate source section: %w", err)
				}
				if cfg.Readme == nil || cfg.Readme.SourceSection == "" {
					return fmt.Errorf("cannot use --generate-source without a configured 'readme.source_section'")
				}

				ulog.Info("Generating source section before sync").
					Field("section", cfg.Readme.SourceSection).
					Log(ctx)
				gen := generator.New(getLogger())
				opts := generator.GenerateOptions{
					Sections: []string{cfg.Readme.SourceSection},
				}
				if err := gen.GenerateWithOptions(cwd, opts); err != nil {
					return fmt.Errorf("failed to generate source section '%s': %w", cfg.Readme.SourceSection, err)
				}
			}

			sync := readme.New(getLogger())

			ulog.Info("Synchronizing README from template and documentation").Log(ctx)
			err = sync.Sync(cwd)
			if err != nil {
				return err
			}

			ulog.Success("README.md synchronized successfully").Log(ctx)
			return nil
		},
	}

	cmd.Flags().BoolVar(&generateSource, "generate-source", false, "Generate the source documentation section before syncing the README")

	return cmd
}