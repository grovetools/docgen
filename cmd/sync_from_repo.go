package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/spf13/cobra"
)

var fromRepoDryRun bool

func newSyncFromRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-repo",
		Short: "Copy docs from repository to notebook",
		Long: `Copies documentation files from the repository's docs/ directory
to the notebook's docgen/docs/ directory for editing/drafting.

This command:
1. Resolves your workspace's notebook docgen/docs directory
2. Copies all .md files from the repository's docs/ directory
3. Reports what was copied

Use this when you want to import existing repository docs into your notebook
for editing and iterating privately before publishing.

Examples:
  docgen sync from-repo              # Copy docs from repository
  docgen sync from-repo --dry-run    # Preview what would be copied`,
		RunE: runSyncFromRepo,
	}

	cmd.Flags().BoolVar(&fromRepoDryRun, "dry-run", false, "Show what would be copied without making changes")

	return cmd
}

func runSyncFromRepo(cmd *cobra.Command, args []string) error {
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
	notebookDocgenDir, err := locator.GetDocgenDir(node)
	if err != nil {
		return fmt.Errorf("could not resolve notebook docgen directory: %w", err)
	}

	// 3. Source and target directories
	sourceDir := filepath.Join(cwd, "docs")
	targetDir := filepath.Join(notebookDocgenDir, "docs")

	// Check if source exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		ulog.Info("No docs directory found in repository").
			Field("path", sourceDir).
			Emit()
		return fmt.Errorf("no docs directory found at %s", sourceDir)
	}

	// 4. List files to copy
	files, err := listMarkdownFiles(sourceDir)
	if err != nil {
		return fmt.Errorf("could not list files in source directory: %w", err)
	}

	if len(files) == 0 {
		ulog.Info("No markdown files found in repository docs directory").
			Field("path", sourceDir).
			Emit()
		return nil
	}

	// 5. Show what will be done
	ulog.Info("Sync plan").
		Field("source", sourceDir).
		Field("target", targetDir).
		Field("files", len(files)).
		Emit()

	if fromRepoDryRun {
		ulog.Info("DRY RUN: No changes will be made").Emit()
		for _, file := range files {
			ulog.Info("Would copy").
				Field("file", file).
				Emit()
		}
		return nil
	}

	// 6. Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("could not create target directory: %w", err)
	}

	// 7. Copy files
	copiedCount := 0
	for _, file := range files {
		srcPath := filepath.Join(sourceDir, file)
		dstPath := filepath.Join(targetDir, file)

		// Create subdirectories if needed
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("could not create directory for %s: %w", dstPath, err)
		}

		// Copy file
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("could not copy %s: %w", file, err)
		}

		ulog.Info("Copied").
			Field("file", file).
			Field("destination", dstPath).
			Emit()
		copiedCount++
	}

	// 8. Success message
	ulog.Success("Sync complete!").
		Field("files_copied", copiedCount).
		Emit()
	ulog.Info("Documentation files have been copied to the notebook").
		Field("target", targetDir).
		Emit()

	return nil
}
