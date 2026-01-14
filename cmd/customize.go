package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	coreConfig "github.com/mattsolo1/grove-core/config"
	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/mattsolo1/grove-core/util/delegation"
	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/mattsolo1/grove-docgen/pkg/recipes"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newCustomizeCmd() *cobra.Command {
	var recipeType string
	
	cmd := &cobra.Command{
		Use:   "customize [subcommand]",
		Short: "Create a customized documentation plan using grove-flow",
		Long: `Creates a Grove Flow plan for interactively customizing and generating documentation.

This command:
1. Reads your docgen.config.yml configuration
2. Creates a Grove Flow plan with the selected recipe type
3. Passes your configuration to the flow plan via recipe variables

Available recipe types:
- agent: Uses AI agents for interactive customization (default)
- prompts: Uses structured prompts for customization

The resulting flow plan will have jobs specific to the chosen recipe type.

Prerequisites:
- Run 'docgen init' first to create docgen.config.yml
- Ensure 'flow' command is available in your PATH

Examples:
  docgen customize                        # Create plan with default agent recipe
  docgen customize --recipe-type agent    # Create plan with agent recipe
  docgen customize --recipe-type prompts  # Create plan with prompts recipe
  flow run                                # Run the plan after creation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			

			// Check if this is the print-recipes subcommand
			if len(args) > 0 && args[0] == "print-recipes" {
				return printRecipes()
			}

			// Load the docgen configuration
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			cfg, err := loadDocgenConfig(cwd)
			if err != nil {
				ulog.Error("Failed to load docgen.config.yml").
					Err(err).
					Emit()
				ulog.Info("Please run 'docgen init' first to create the configuration file").Emit()
				return err
			}

			// Validate recipe type
			var recipeName string
			switch recipeType {
			case "agent":
				recipeName = "docgen-customize-agent"
			case "prompts":
				recipeName = "docgen-customize-prompts"
			default:
				ulog.Error("Invalid recipe type").
					Field("recipe_type", recipeType).
					Emit()
				ulog.Info("Valid options are: agent, prompts").Emit()
				return fmt.Errorf("invalid recipe type: %s", recipeType)
			}
			
			// Determine the plan name
			projectName := filepath.Base(cwd)
			planName := fmt.Sprintf("%s-%s", recipeName, projectName)
			
			// Build the flow command arguments
			args = []string{
				"plan", "init", planName,
				"--recipe", recipeName,
				"--recipe-cmd", "docgen recipe print",
			}
			
			// Add recipe variables from the configuration
			if cfg.Settings.Model != "" {
				args = append(args, "--recipe-vars", fmt.Sprintf("model=%s", cfg.Settings.Model))
			} else {
				args = append(args, "--recipe-vars", "model=gemini-1.5-flash-latest")
			}

			if cfg.Settings.RulesFile != "" {
				// The rules file is relative to docs/, so prepend docs/ for the full path
				rulesPath := filepath.Join("docs", cfg.Settings.RulesFile)
				args = append(args, "--recipe-vars", fmt.Sprintf("rules_file=%s", rulesPath))
			} else {
				args = append(args, "--recipe-vars", "rules_file=docs/docs.rules")
			}

			if cfg.Settings.OutputDir != "" {
				args = append(args, "--recipe-vars", fmt.Sprintf("output_dir=%s", cfg.Settings.OutputDir))
			} else {
				args = append(args, "--recipe-vars", "output_dir=docs")
			}

			// Resolve and add prompts directory from notebook
			if node, err := workspace.GetProjectByPath(cwd); err == nil {
				if coreCfg, err := coreConfig.LoadDefault(); err == nil {
					locator := workspace.NewNotebookLocator(coreCfg)
					if promptsDir, err := locator.GetDocgenPromptsDir(node); err == nil {
						args = append(args, "--recipe-vars", fmt.Sprintf("prompts_dir=%s", promptsDir))
						log.Debugf("Using prompts directory from notebook: %s", promptsDir)
					}
				}
			}
			
			ulog.Info("Creating customization plan").
				Field("plan_name", planName).
				Field("recipe_type", recipeType).
				Emit()
			log.Debugf("Running: flow %v", args)

			// Execute the flow command
			cmdArgs := append([]string{"flow"}, args...)
			flowCmd := delegation.Command(cmdArgs[0], cmdArgs[1:]...)
			flowCmd.Stdout = os.Stdout
			flowCmd.Stderr = os.Stderr
			flowCmd.Stdin = os.Stdin

			if err := flowCmd.Run(); err != nil {
				return fmt.Errorf("failed to create flow plan: %w", err)
			}

			ulog.Success("Successfully created customization plan").
				Field("plan_name", planName).
				Field("recipe_type", recipeType).
				Field("location", fmt.Sprintf("plans/%s", planName)).
				Emit()

			ulog.Info("Next steps").
				PrettyOnly().
				Pretty("\nNext steps:\n  1. Run 'flow run' to start the customization process").
				Emit()

			if recipeType == "agent" {
				ulog.Info("Agent customization").
					PrettyOnly().
					Pretty("  2. The agent will interactively help you customize and generate documentation").
					Emit()
			} else {
				ulog.Info("Prompts customization").
					PrettyOnly().
					Pretty("  2. Follow the prompts to customize your documentation structure\n  3. The generation job will create documentation based on your customizations").
					Emit()
			}

			return nil
		},
	}
	
	// Add flags
	cmd.Flags().StringVarP(&recipeType, "recipe-type", "r", "agent", "Recipe type to use: 'agent' or 'prompts'")
	
	return cmd
}

// loadDocgenConfig loads the docgen.config.yml from the specified directory
func loadDocgenConfig(dir string) (*config.DocgenConfig, error) {
	configPath := filepath.Join(dir, "docs", config.ConfigFileName)
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("docgen.config.yml not found at %s", configPath)
		}
		return nil, fmt.Errorf("failed to read docgen.config.yml: %w", err)
	}
	
	var cfg config.DocgenConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse docgen.config.yml: %w", err)
	}
	
	return &cfg, nil
}

// printRecipes prints all available recipes in JSON format (for grove-flow integration)
func printRecipes() error {
	
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
}

func loadDocgenRecipe(recipeName string, embedFS fs.FS) (recipes.RecipeDefinition, error) {
	description := getRecipeDescription(recipeName)
	recipe := recipes.RecipeDefinition{
		Description: description,
		Jobs:        make(map[string]string),
	}

	// Walk through the embedded filesystem to find all .md files
	err := fs.WalkDir(embedFS, fmt.Sprintf("builtin/%s", recipeName), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Read the file content
		content, err := fs.ReadFile(embedFS, path)
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

func getRecipeDescription(recipeName string) string {
	switch recipeName {
	case "docgen-customize-agent":
		return "Generate comprehensive project documentation using AI agents for interactive customization"
	case "docgen-customize-prompts":
		return "Generate comprehensive project documentation using structured prompts for customization"
	default:
		return "Generate comprehensive project documentation with customizable structure"
	}
}