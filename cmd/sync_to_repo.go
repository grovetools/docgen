package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	docgenConfig "github.com/grovetools/docgen/pkg/config"
	"github.com/spf13/cobra"
)

var (
	toRepoDryRun          bool
	toRepoForce           bool
	toRepoIncludeAllDraft bool
)

func newSyncToRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "to-repo",
		Short: "Copy generated docs from notebook to repository",
		Long: `Copies documentation files from the notebook's docgen/docs/ directory
to the repository's docs/ directory for publishing/committing.

This command:
1. Resolves your workspace's notebook docgen/docs directory
2. Copies all .md files to the repository's docs/ directory
3. Optionally copies images, asciicasts, and other assets
4. Reports what was copied

Use this when you're ready to finalize and publish documentation changes.

Examples:
  docgen sync to-repo              # Copy docs to repository
  docgen sync to-repo --dry-run    # Preview what would be copied
  docgen sync to-repo --force      # Overwrite existing files without prompting`,
		RunE: runSyncToRepo,
	}

	cmd.Flags().BoolVar(&toRepoDryRun, "dry-run", false, "Show what would be copied without making changes")
	cmd.Flags().BoolVar(&toRepoForce, "force", false, "Overwrite existing files without prompting")
	cmd.Flags().BoolVar(&toRepoIncludeAllDraft, "include-draft", false, "Include draft sections (by default only 'production' status sections are synced)")

	return cmd
}

func runSyncToRepo(cmd *cobra.Command, args []string) error {
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

	// 3. Load docgen config to get section status
	cfg, _, err := docgenConfig.LoadWithNotebook(cwd)
	if err != nil {
		return fmt.Errorf("could not load docgen config: %w", err)
	}

	// 4. Build list of files to sync based on status
	var filesToSync []string
	var skippedDraft []string
	var skippedDev []string

	for _, section := range cfg.Sections {
		status := section.GetStatus()

		// Only sync "production" status sections (unless --include-draft)
		if status == docgenConfig.StatusProduction || toRepoIncludeAllDraft {
			filesToSync = append(filesToSync, section.Output)
		} else if status == docgenConfig.StatusDraft {
			skippedDraft = append(skippedDraft, section.Output)
		} else if status == docgenConfig.StatusDev {
			skippedDev = append(skippedDev, section.Output)
		}
	}

	// 5. Source and target directories
	sourceDir := filepath.Join(notebookDocgenDir, "docs")
	targetDir := filepath.Join(cwd, "docs")

	// Check if source exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		ulog.Info("No docs directory found in notebook").
			Field("path", sourceDir).
			Emit()
		ulog.Info("Run 'docgen generate' first to create documentation in the notebook").Emit()
		return nil
	}

	// 6. Show status summary
	ulog.Info("Documentation status summary").
		Field("production", len(filesToSync)).
		Field("dev", len(skippedDev)).
		Field("draft", len(skippedDraft)).
		Emit()

	if len(filesToSync) > 0 {
		ulog.Success("Files to sync to repository (production)").Emit()
		for _, file := range filesToSync {
			ulog.Info("  → " + file).PrettyOnly().Emit()
		}
	}

	if len(skippedDev) > 0 {
		ulog.Info("Dev status (visible on dev website only)").Emit()
		for _, file := range skippedDev {
			ulog.Info("  ○ " + file).PrettyOnly().Emit()
		}
	}

	if len(skippedDraft) > 0 {
		ulog.Info("Draft status (notebook only, not synced)").Emit()
		for _, file := range skippedDraft {
			ulog.Info("  • " + file).PrettyOnly().Emit()
		}
	}

	if len(filesToSync) == 0 {
		ulog.Info("No production-ready files to sync").Emit()
		ulog.Info("Tip: Set status: production in docgen.config.yml to sync files").PrettyOnly().Emit()
		return nil
	}

	// 7. Show sync details
	ulog.Info("Sync plan").
		Field("source", sourceDir).
		Field("target", targetDir).
		Emit()

	if toRepoDryRun {
		ulog.Info("DRY RUN: No changes will be made").Emit()
		return nil
	}

	// 8. Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("could not create target directory: %w", err)
	}

	// 9. Copy files
	copiedCount := 0
	for _, file := range filesToSync {
		srcPath := filepath.Join(sourceDir, file)
		dstPath := filepath.Join(targetDir, file)

		// Check if file exists and prompt if needed (unless --force)
		if !toRepoForce {
			if _, err := os.Stat(dstPath); err == nil {
				ulog.Info("File exists in target, overwriting").
					Field("file", file).
					Emit()
			}
		}

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

	// 10. Success message
	ulog.Success("Sync complete!").
		Field("files_copied", copiedCount).
		Emit()
	ulog.Info("Documentation files have been copied to the repository").
		Field("target", targetDir).
		Emit()

	return nil
}

// listMarkdownFiles returns all .md files in a directory (recursively)
func listMarkdownFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".md" {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
