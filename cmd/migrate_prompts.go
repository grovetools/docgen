package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	coreConfig "github.com/mattsolo1/grove-core/config"
	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var migrateDryRun bool

func newMigratePromptsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-prompts",
		Short: "Migrate prompts from docs/prompts to notebook workspace",
		Long: `Moves prompt files from the local docs/prompts directory to the
associated notebook workspace and updates docgen.config.yml.

The command will:
1. Resolve your workspace's notebook location
2. Copy prompt files from docs/prompts/ to the notebook's docgen directory
3. Update docgen.config.yml to use basenames only (e.g., "01-overview.md" instead of "prompts/01-overview.md")
4. Optionally delete the old docs/prompts directory

Use --dry-run to see what would be changed without making modifications.

Examples:
  docgen migrate-prompts          # Run the migration
  docgen migrate-prompts --dry-run   # Preview changes without applying them`,
		RunE: runMigratePrompts,
	}

	cmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

func runMigratePrompts(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// 1. Resolve workspace
	node, err := workspace.GetProjectByPath(cwd)
	if err != nil {
		ulog.Error("Could not resolve workspace").
			Err(err).
			Emit()
		ulog.Info("Ensure this project is in a configured grove in ~/.config/grove/grove.yml").Emit()
		return fmt.Errorf("could not resolve workspace: %w", err)
	}

	// 2. Resolve notebook prompt directory
	coreCfg, err := coreConfig.LoadDefault()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	locator := workspace.NewNotebookLocator(coreCfg)
	targetDir, err := locator.GetDocgenPromptsDir(node)
	if err != nil {
		return fmt.Errorf("could not resolve notebook prompts directory: %w", err)
	}

	// 3. Check if source prompts exist
	sourceDir := "./docs/prompts"
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		ulog.Info("No docs/prompts directory found. Nothing to migrate.").Emit()
		return nil
	}

	// 4. Check if already migrated (idempotency)
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("could not read source directory: %w", err)
	}

	// Filter out directories, only count files
	var fileEntries []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			fileEntries = append(fileEntries, entry)
		}
	}

	if len(fileEntries) == 0 {
		ulog.Info("docs/prompts directory is empty. Already migrated?").Emit()
		return nil
	}

	// 5. Show what will be done
	ulog.Info("Migration plan").
		Field("source", sourceDir).
		Field("target", targetDir).
		Field("files", len(fileEntries)).
		Emit()

	if migrateDryRun {
		ulog.Info("DRY RUN: No changes will be made").Emit()
		for _, entry := range fileEntries {
			ulog.Info("Would copy").
				Field("file", entry.Name()).
				Emit()
		}
		ulog.Info("Would update docgen.config.yml to use basenames only").Emit()
		return nil
	}

	// 6. Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("could not create target directory: %w", err)
	}

	// 7. Copy files
	for _, entry := range fileEntries {
		srcPath := filepath.Join(sourceDir, entry.Name())
		dstPath := filepath.Join(targetDir, entry.Name())

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("could not read %s: %w", srcPath, err)
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("could not write %s: %w", dstPath, err)
		}

		ulog.Info("Copied").
			Field("file", entry.Name()).
			Field("destination", dstPath).
			Emit()
	}

	// 8. Update config file
	configPath := "./docs/docgen.config.yml"
	if err := updateConfigFilePromptPaths(configPath); err != nil {
		ulog.Warn("Could not update config file").
			Err(err).
			Emit()
		ulog.Warn("You may need to manually update prompt paths to basenames only").Emit()
	} else {
		ulog.Info("Updated docgen.config.yml").Emit()
	}

	// 9. Offer to delete old directory
	ulog.Success("Migration complete!").Emit()
	ulog.Info("Next steps").
		PrettyOnly().
		Pretty(fmt.Sprintf("\nYou can now delete the old prompts directory:\n  rm -rf %s", sourceDir)).
		Emit()

	return nil
}

// updateConfigFilePromptPaths updates prompt: fields to basename only
func updateConfigFilePromptPaths(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var cfg config.DocgenConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Update each section's prompt field to use basename only
	modified := false
	for i := range cfg.Sections {
		originalPrompt := cfg.Sections[i].Prompt
		basename := filepath.Base(originalPrompt)

		// Only update if it has a directory component
		if originalPrompt != basename {
			cfg.Sections[i].Prompt = basename
			modified = true
			log.Debugf("Updated prompt path: %s -> %s", originalPrompt, basename)
		}
	}

	// Only write back if we made changes
	if !modified {
		log.Debug("No prompt paths needed updating")
		return nil
	}

	// Write back
	newData, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, newData, 0644)
}
