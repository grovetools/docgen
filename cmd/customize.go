package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mattsolo1/grove-core/cli"
	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newCustomizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "customize",
		Short: "Create a customized documentation plan using grove-flow",
		Long: `Creates a Grove Flow plan for interactively customizing and generating documentation.

This command:
1. Reads your docgen.config.yml configuration
2. Creates a Grove Flow plan with the docgen-customize recipe
3. Passes your configuration to the flow plan via recipe variables

The resulting flow plan will have two jobs:
- A chat job to help you customize your documentation structure
- An interactive agent job to generate the documentation

Prerequisites:
- Run 'docgen init' first to create docgen.config.yml
- Ensure 'flow' command is available in your PATH

Example:
  docgen customize                # Create a customization plan
  flow run                         # Run the plan after creation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			
			// Load the docgen configuration
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			
			cfg, err := loadDocgenConfig(cwd)
			if err != nil {
				logger.Error("Failed to load docgen.config.yml")
				logger.Info("Please run 'docgen init' first to create the configuration file")
				return err
			}
			
			// Determine the plan name
			projectName := filepath.Base(cwd)
			planName := fmt.Sprintf("docgen-customize-%s", projectName)
			
			// Build the flow command arguments
			args = []string{
				"plan", "init", planName,
				"--recipe", "docgen-customize",
			}
			
			// Add recipe variables from the configuration
			if cfg.Settings.Model != "" {
				args = append(args, "--recipe-vars", fmt.Sprintf("model=%s", cfg.Settings.Model))
			} else {
				args = append(args, "--recipe-vars", "model=gemini-1.5-flash-latest")
			}
			
			if cfg.Settings.RulesFile != "" {
				args = append(args, "--recipe-vars", fmt.Sprintf("rules_file=%s", cfg.Settings.RulesFile))
			} else {
				args = append(args, "--recipe-vars", "rules_file=docs.rules")
			}
			
			if cfg.Settings.OutputDir != "" {
				args = append(args, "--recipe-vars", fmt.Sprintf("output_dir=%s", cfg.Settings.OutputDir))
			} else {
				args = append(args, "--recipe-vars", "output_dir=docs")
			}
			
			logger.Infof("Creating customization plan: %s", planName)
			logger.Debugf("Running: flow %v", args)
			
			// Execute the flow command
			flowCmd := exec.Command("flow", args...)
			flowCmd.Stdout = os.Stdout
			flowCmd.Stderr = os.Stderr
			flowCmd.Stdin = os.Stdin
			
			if err := flowCmd.Run(); err != nil {
				return fmt.Errorf("failed to create flow plan: %w", err)
			}
			
			logger.Info("")
			logger.Infof("✅ Successfully created customization plan in 'plans/%s'", planName)
			logger.Info("")
			logger.Info("Next steps:")
			logger.Info("  1. Run 'flow run' to start the customization process")
			logger.Info("  2. The first job will help you define your documentation structure")
			logger.Info("  3. The second job will generate the documentation based on your plan")
			
			return nil
		},
	}
	
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