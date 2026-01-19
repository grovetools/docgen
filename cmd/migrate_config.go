package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/docgen/pkg/config"
	"github.com/spf13/cobra"
)

var migrateConfigDryRun bool

func newMigrateConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-config",
		Short: "Migrate docgen config from docs/ to notebook workspace",
		Long: `Moves the docgen.config.yml file from docs/ to the notebook's docgen directory.

The command will:
1. Resolve your workspace's notebook docgen location
2. Copy docgen.config.yml from docs/ to the notebook's docgen directory
3. Keep the original file in docs/ (no deletion)

Once migrated, 'docgen generate' will automatically use the notebook config.

Use --dry-run to see what would be changed without making modifications.

Examples:
  docgen migrate-config           # Run the migration
  docgen migrate-config --dry-run # Preview changes without applying them`,
		RunE: runMigrateConfig,
	}

	cmd.Flags().BoolVar(&migrateConfigDryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

func runMigrateConfig(cmd *cobra.Command, args []string) error {
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

	// 2. Resolve notebook docgen directory
	coreCfg, err := coreConfig.LoadDefault()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	locator := workspace.NewNotebookLocator(coreCfg)
	targetDir, err := locator.GetDocgenDir(node)
	if err != nil {
		return fmt.Errorf("could not resolve notebook docgen directory: %w", err)
	}

	// 3. Check if source config exists
	sourceFile := "./docs/docgen.config.yml"
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		ulog.Info("No docs/docgen.config.yml found. Nothing to migrate.").Emit()
		return nil
	}

	// 4. Check if already migrated (target exists)
	targetFile := filepath.Join(targetDir, config.ConfigFileName)
	if _, err := os.Stat(targetFile); err == nil {
		ulog.Info("Config already exists in notebook").
			Field("path", targetFile).
			Emit()
		ulog.Info("Migration already complete or config was created directly in notebook").Emit()
		return nil
	}

	// 5. Show what will be done
	ulog.Info("Migration plan").
		Field("source", sourceFile).
		Field("target", targetFile).
		Emit()

	if migrateConfigDryRun {
		ulog.Info("DRY RUN: No changes will be made").Emit()
		ulog.Info("Would copy config file to notebook").Emit()
		return nil
	}

	// 6. Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("could not create target directory: %w", err)
	}

	// 7. Copy file
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", sourceFile, err)
	}

	if err := os.WriteFile(targetFile, data, 0644); err != nil {
		return fmt.Errorf("could not write %s: %w", targetFile, err)
	}

	ulog.Info("Copied config file").
		Field("source", sourceFile).
		Field("destination", targetFile).
		Emit()

	// 8. Success message
	ulog.Success("Migration complete!").Emit()
	ulog.Info("Future 'docgen generate' runs will use the notebook config").Emit()
	ulog.Info("Next steps").
		PrettyOnly().
		Pretty(fmt.Sprintf("\nThe original config in docs/ can be kept for reference or removed:\n  rm %s", sourceFile)).
		Emit()

	return nil
}
