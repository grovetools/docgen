package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/mattsolo1/grove-docgen/pkg/recipes"
	"github.com/spf13/cobra"
)

func newRecipeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Manage and display documentation recipes",
		Long:  "Commands for working with documentation generation recipes",
	}

	cmd.AddCommand(newRecipePrintCmd())

	return cmd
}

func newRecipePrintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print available recipes in JSON format",
		Long:  "Print all available documentation recipes in a format suitable for grove-flow integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			
			collection := make(recipes.RecipeCollection)

			// Load the docgen-customize-agent recipe
			agentRecipe, err := loadDocgenRecipe("docgen-customize-agent", recipes.DocgenCustomizeAgentFS)
			if err != nil {
				return fmt.Errorf("failed to load docgen-customize-agent recipe: %w", err)
			}
			collection["docgen-customize-agent"] = agentRecipe

			// Load the docgen-customize-prompts recipe
			promptsRecipe, err := loadDocgenRecipe("docgen-customize-prompts", recipes.DocgenCustomizePromptsFS)
			if err != nil {
				return fmt.Errorf("failed to load docgen-customize-prompts recipe: %w", err)
			}
			collection["docgen-customize-prompts"] = promptsRecipe

			// Load the add-readme-template recipe
			readmeRecipe, err := loadDocgenRecipe("add-readme-template", recipes.AddReadmeTemplateFS)
			if err != nil {
				return fmt.Errorf("failed to load add-readme-template recipe: %w", err)
			}
			collection["add-readme-template"] = readmeRecipe

			// Output as JSON
			jsonData, err := json.MarshalIndent(collection, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal recipes to JSON: %w", err)
			}

			ulog.Info("Recipe collection").
				Field("recipe_count", len(collection)).
				PrettyOnly().
				Pretty(string(jsonData)).
				Emit()

			return nil
		},
	}

	return cmd
}

