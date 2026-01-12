package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

//go:embed all:templates
var templatesFS embed.FS

// InitOptions holds configuration options for the init command
type InitOptions struct {
	Model                string
	RegenerationMode     string
	RulesFile            string
	StructuredOutputFile string
	SystemPrompt         string
	OutputDir            string
}

// Init scaffolds a new docgen configuration in the current directory with default options.
func Init(projectType string, logger *logrus.Logger) error {
	return InitWithOptions(projectType, InitOptions{}, logger)
}

// InitWithOptions scaffolds a new docgen configuration with custom options.
func InitWithOptions(projectType string, opts InitOptions, logger *logrus.Logger) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	docsDir := filepath.Join(cwd, "docs")
	promptsDir := filepath.Join(docsDir, "prompts")

	// 1. Check for existing config to prevent overwrite
	configDest := filepath.Join(docsDir, "docgen.config.yml")
	if _, err := os.Stat(configDest); err == nil {
		return fmt.Errorf("docgen configuration already exists at %s", configDest)
	}

	// 2. Create destination directories
	logger.Debugf("Creating directory: %s", promptsDir)
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// 3. Copy and customize config file
	configSrcPath := filepath.Join("templates", projectType, "docgen.config.yml")
	logger.Debugf("Copying %s to %s", configSrcPath, configDest)
	if err := copyAndCustomizeConfig(configSrcPath, configDest, opts); err != nil {
		return err
	}
	logger.Infof("✓ Created configuration file: %s", filepath.Join("docs", "docgen.config.yml"))

	// 4. Copy README.md.tpl to docs directory
	readmeTplSrc := filepath.Join("templates", projectType, "docs", "README.md.tpl")
	readmeTplDest := filepath.Join(docsDir, "README.md.tpl")
	if _, err := os.Stat(readmeTplDest); os.IsNotExist(err) {
		if err := copyFileFromFS(readmeTplSrc, readmeTplDest); err != nil {
			return fmt.Errorf("failed to copy README.md.tpl: %w", err)
		}
		logger.Infof("✓ Created README template: %s", filepath.Join("docs", "README.md.tpl"))
	}

	// 5. Create rules file if specified
	if opts.RulesFile != "" {
		rulesPath := filepath.Join(docsDir, opts.RulesFile)
		// Only create if it doesn't exist
		if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
			// Create a basic rules file
			rulesContent := `*
`
			if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
				return fmt.Errorf("failed to create rules file: %w", err)
			}
			logger.Infof("✓ Created rules file: %s", filepath.Join("docs", opts.RulesFile))
		} else if err == nil {
			logger.Infof("✓ Rules file already exists: %s", filepath.Join("docs", opts.RulesFile))
		}
	}

	// 6. Copy prompt files
	promptsSrcDir := filepath.Join("templates", projectType, "prompts")
	entries, err := templatesFS.ReadDir(promptsSrcDir)
	if err != nil {
		return fmt.Errorf("failed to read embedded prompts directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			src := filepath.Join(promptsSrcDir, entry.Name())
			dest := filepath.Join(promptsDir, entry.Name())
			logger.Debugf("Copying %s to %s", src, dest)
			if err := copyFileFromFS(src, dest); err != nil {
				return err
			}
			logger.Infof("✓ Created prompt file: %s", filepath.Join("docs", "prompts", entry.Name()))
		}
	}

	logger.Info("✅ Docgen initialized successfully.")
	logger.Info("✓ Created docs/prompts/ with starter prompts")
	logger.Info("")
	logger.Info("💡 If you use grove-notebook, run 'docgen migrate-prompts' to move prompts to your notebook")
	logger.Info("")
	logger.Info("   Next steps: 1. Edit docs/docgen.config.yml to match your project.")
	if opts.RulesFile != "" {
		logger.Infof("               2. Review and customize the rules in docs/%s.", opts.RulesFile)
	}
	logger.Info("               3. Review and customize the prompts in docs/prompts/.")
	logger.Info("               4. Run 'make generate-docs' to create documentation and sync your README.")

	return nil
}

func copyFileFromFS(src, dest string) error {
	content, err := templatesFS.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read embedded file %s: %w", src, err)
	}
	if err := os.WriteFile(dest, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", dest, err)
	}
	return nil
}

// copyAndCustomizeConfig copies the config template and applies any custom options
func copyAndCustomizeConfig(src, dest string, opts InitOptions) error {
	content, err := templatesFS.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read embedded file %s: %w", src, err)
	}

	// If no options are provided, just write the file as-is
	if opts == (InitOptions{}) {
		return os.WriteFile(dest, content, 0644)
	}

	// Parse the YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("failed to parse config template: %w", err)
	}

	// Get or create settings section
	settings, ok := config["settings"].(map[string]interface{})
	if !ok {
		settings = make(map[string]interface{})
		config["settings"] = settings
	}

	// Apply custom options
	if opts.Model != "" {
		settings["model"] = opts.Model
	}
	if opts.RegenerationMode != "" {
		settings["regeneration_mode"] = opts.RegenerationMode
	}
	if opts.RulesFile != "" {
		settings["rules_file"] = opts.RulesFile
	}
	if opts.StructuredOutputFile != "" {
		settings["structured_output_file"] = opts.StructuredOutputFile
	}
	if opts.SystemPrompt != "" {
		settings["system_prompt"] = opts.SystemPrompt
	}
	if opts.OutputDir != "" {
		settings["output_dir"] = opts.OutputDir
	}

	// Marshal back to YAML
	updatedContent, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	// Add the schema comment back at the top
	finalContent := "# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json\n" + string(updatedContent)
	
	// Clean up the YAML formatting
	finalContent = strings.ReplaceAll(finalContent, "\n    ", "\n  ")

	return os.WriteFile(dest, []byte(finalContent), 0644)
}