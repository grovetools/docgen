package cmd

import (
	"github.com/mattsolo1/grove-core/cli"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {
	rootCmd = cli.NewStandardCommand("docgen", "LLM-powered, workspace-aware documentation generator.")

	// Add commands
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newAggregateCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newRegenJSONCmd())
	rootCmd.AddCommand(newCustomizeCmd())
	rootCmd.AddCommand(newRecipeCmd())
	rootCmd.AddCommand(newSyncReadmeCmd())
	rootCmd.AddCommand(newSchemaCmd())
	rootCmd.AddCommand(newMigratePromptsCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
