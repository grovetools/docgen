package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

//go:embed all:templates
var templatesFS embed.FS

// Init scaffolds a new docgen configuration in the current directory.
func Init(projectType string, logger *logrus.Logger) error {
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

	// 3. Copy config file
	configSrcPath := filepath.Join("templates", projectType, "docgen.config.yml")
	logger.Debugf("Copying %s to %s", configSrcPath, configDest)
	if err := copyFileFromFS(configSrcPath, configDest); err != nil {
		return err
	}
	logger.Infof("✓ Created configuration file: %s", filepath.Join("docs", "docgen.config.yml"))

	// 4. Copy prompt files
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
	logger.Info("   Next steps: 1. Edit docs/docgen.config.yml to match your project.")
	logger.Info("               2. Review and customize the prompts in docs/prompts/.")
	logger.Info("               3. Run 'docgen generate' to create your documentation.")

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