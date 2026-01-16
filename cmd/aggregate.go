package cmd

import (
	"github.com/grovetools/docgen/pkg/aggregator"
	"github.com/spf13/cobra"
)

func newAggregateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Generate and aggregate documentation from all workspace packages",
		Long:  `Discovers all packages in the workspace, generates documentation for each enabled package, and aggregates the results into an output directory with a manifest.json file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir, _ := cmd.Flags().GetString("output-dir")

			agg := aggregator.New(getLogger())
			return agg.Aggregate(outputDir)
		},
	}
	cmd.Flags().StringP("output-dir", "o", "dist", "Directory to save the aggregated documentation")
	return cmd
}