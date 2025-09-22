package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

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

			// Load the docgen-customize recipe
			recipe, err := loadDocgenCustomizeRecipe()
			if err != nil {
				return fmt.Errorf("failed to load docgen-customize recipe: %w", err)
			}
			collection["docgen-customize"] = recipe

			// Output as JSON
			jsonData, err := json.MarshalIndent(collection, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal recipes to JSON: %w", err)
			}
			fmt.Println(string(jsonData))

			return nil
		},
	}

	return cmd
}

func loadDocgenCustomizeRecipe() (recipes.RecipeDefinition, error) {
	recipe := recipes.RecipeDefinition{
		Description: "Generate comprehensive project documentation with customizable structure",
		Jobs:        make(map[string]string),
	}

	// Walk through the embedded filesystem to find all .md files
	err := fs.WalkDir(recipes.DocgenCustomizeFS, "builtin/docgen-customize", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Read the file content
		content, err := fs.ReadFile(recipes.DocgenCustomizeFS, path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Get the filename (e.g., "01-customize-docs.md")
		filename := filepath.Base(path)
		recipe.Jobs[filename] = string(content)

		return nil
	})

	if err != nil {
		return recipe, fmt.Errorf("failed to walk embedded files: %w", err)
	}

	// Ensure we have the expected files
	if len(recipe.Jobs) == 0 {
		return recipe, fmt.Errorf("no recipe files found")
	}

	return recipe, nil
}