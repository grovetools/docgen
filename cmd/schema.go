package cmd

import "github.com/spf13/cobra"

func newSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage and process JSON schemas",
		Long:  "Provides tools for generating, enriching, and documenting JSON schemas.",
	}

	cmd.AddCommand(newSchemaEnrichCmd())
	cmd.AddCommand(newSchemaGenerateCmd())

	return cmd
}
