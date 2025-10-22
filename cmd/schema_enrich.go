package cmd

import (
	"fmt"
	"os"

	"github.com/mattsolo1/grove-docgen/pkg/schema_enricher"
	"github.com/spf13/cobra"
)

func newSchemaEnrichCmd() *cobra.Command {
	var inPlace bool

	cmd := &cobra.Command{
		Use:   "enrich <path/to/schema.json>",
		Short: "Enrich a JSON schema with AI-generated descriptions",
		Long: `Analyzes a JSON schema file, identifies properties lacking descriptions, and uses an LLM with project context to generate and insert those descriptions.

The enriched schema is printed to stdout unless the --in-place flag is used.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaPath := args[0]
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			enricher := schema_enricher.New(getLogger())
			return enricher.Enrich(cwd, schemaPath, inPlace)
		},
	}

	cmd.Flags().BoolVar(&inPlace, "in-place", false, "Modify the schema file directly instead of printing to stdout")

	return cmd
}
