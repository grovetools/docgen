package cmd

import (
	"github.com/mattsolo1/grove-core/logging"
	"github.com/mattsolo1/grove-docgen/pkg/aggregator"
	"github.com/spf13/cobra"
)

func newAggregateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Generate and aggregate documentation from all workspace packages",
		Long:  `Discovers all packages in the workspace, generates documentation for each enabled package, and aggregates the results into an output directory with a manifest.json file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.NewLogger("grove-docgen")
			outputDir, _ := cmd.Flags().GetString("output-dir")

			agg := aggregator.New(logger.Logger)
			return agg.Aggregate(outputDir)
		},
	}
	cmd.Flags().StringP("output-dir", "o", "dist", "Directory to save the aggregated documentation")
	return cmd
}