package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/docgen/pkg/config"
)

// generateFromConcept copies documentation from an nb concept directory.
// It copies all .md files found in the concept directory to the output location,
// replacing any existing frontmatter with proper Astro-compatible frontmatter.
// The source field should contain the concept ID, optionally prefixed with a workspace name
// (e.g., "my-concept" for the current workspace or "core:my-concept" for cross-workspace).
// The output field should specify the base directory (e.g., "concepts/").
func (g *Generator) generateFromConcept(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Copying concept docs: %s", section.Source)

	if section.Source == "" {
		return fmt.Errorf("section type 'nb_concept' requires a 'source' (concept ID)")
	}

	// Parse source for optional workspace prefix (e.g., "core:my-concept")
	conceptID := section.Source
	targetWorkspace := ""
	if strings.Contains(section.Source, ":") {
		parts := strings.SplitN(section.Source, ":", 2)
		targetWorkspace = parts[0]
		conceptID = parts[1]
	}

	// 1. Resolve Workspace Node
	var node *workspace.WorkspaceNode
	var err error

	if targetWorkspace != "" {
		// Cross-workspace: find the target workspace by searching all projects
		allProjects, err := workspace.GetProjects(g.logger)
		if err != nil {
			return fmt.Errorf("could not discover workspaces: %w", err)
		}

		for _, project := range allProjects {
			if project.Name == targetWorkspace {
				node = project
				break
			}
		}

		if node == nil {
			return fmt.Errorf("could not find target workspace '%s'", targetWorkspace)
		}
	} else {
		// Current workspace: resolve from package directory
		node, err = workspace.GetProjectByPath(packageDir)
		if err != nil {
			return fmt.Errorf("could not resolve workspace for %s: %w", packageDir, err)
		}
	}

	// 2. Resolve Concepts Directory
	coreCfg, err := coreConfig.LoadDefault()
	if err != nil {
		return fmt.Errorf("could not load core config: %w", err)
	}

	locator := workspace.NewNotebookLocator(coreCfg)

	// Get the docgen directory, then navigate up to the workspace level and into concepts
	docgenDir, err := locator.GetDocgenDir(node)
	if err != nil {
		return fmt.Errorf("could not resolve docgen directory: %w", err)
	}

	// docgenDir is {notebook_root}/workspaces/{name}/docgen
	// so concepts is {notebook_root}/workspaces/{name}/concepts
	workspaceDir := filepath.Dir(docgenDir)
	conceptDir := filepath.Join(workspaceDir, "concepts", conceptID)
	if _, err := os.Stat(conceptDir); os.IsNotExist(err) {
		return fmt.Errorf("concept directory not found: %s", conceptDir)
	}

	// 3. Find all .md files in the concept directory
	entries, err := os.ReadDir(conceptDir)
	if err != nil {
		return fmt.Errorf("could not read concept directory: %w", err)
	}

	var mdFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			mdFiles = append(mdFiles, entry.Name())
		}
	}

	if len(mdFiles) == 0 {
		return fmt.Errorf("no .md files found in concept directory: %s", conceptDir)
	}

	// Determine output directory
	// If output ends with .md, use the directory part; otherwise use as-is
	outputDir := section.Output
	if strings.HasSuffix(outputDir, ".md") {
		outputDir = filepath.Dir(outputDir)
	}
	// Ensure it ends with the concept ID for proper nesting
	if !strings.HasSuffix(outputDir, conceptID) {
		outputDir = filepath.Join(outputDir, conceptID)
	}

	// Get package name from config title (e.g., "flow")
	pkgName := cfg.Title
	category := cfg.Category

	// 4. Copy each .md file to output with proper frontmatter
	for i, mdFile := range mdFiles {
		srcPath := filepath.Join(conceptDir, mdFile)
		content, err := os.ReadFile(srcPath)
		if err != nil {
			g.logger.Warnf("Could not read %s: %v", srcPath, err)
			continue
		}

		// Strip existing frontmatter and get body
		body := stripFrontmatter(string(content))

		// Generate title from filename (e.g., "cli-output-destinations.md" -> "CLI Output Destinations")
		title := formatTitle(strings.TrimSuffix(mdFile, ".md"))

		// Create new frontmatter
		// Order: base order from section + file index
		order := section.Order*100 + i + 1
		newFrontmatter := fmt.Sprintf(`---
title: "%s"
package: "%s"
category: "%s"
order: %d
---

`, title, pkgName, category, order)

		// Combine new frontmatter with body
		newContent := newFrontmatter + body

		// Write output
		outputPath := filepath.Join(outputBaseDir, outputDir, mdFile)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			g.logger.Errorf("Failed to create output directory for %s: %v", mdFile, err)
			continue
		}
		if err := os.WriteFile(outputPath, []byte(newContent), 0644); err != nil {
			g.logger.Errorf("Failed to write output for %s: %v", mdFile, err)
			continue
		}

		g.logger.Infof("Copied concept doc: %s", outputPath)
		ulog.Success("Copied concept doc").
			Field("file", mdFile).
			Field("path", outputPath).
			Emit()
	}

	return nil
}

// stripFrontmatter removes YAML frontmatter from markdown content
func stripFrontmatter(content string) string {
	// Match frontmatter: starts with ---, ends with ---
	re := regexp.MustCompile(`(?s)^---\n.*?\n---\n*`)
	return re.ReplaceAllString(content, "")
}

// formatTitle converts a filename to a title
// e.g., "cli-output-destinations" -> "CLI Output Destinations"
func formatTitle(name string) string {
	// Split by dashes and underscores
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	})

	// Capitalize each part, with special handling for common acronyms
	acronyms := map[string]string{
		"cli": "CLI",
		"tui": "TUI",
		"api": "API",
		"ui":  "UI",
		"id":  "ID",
		"llm": "LLM",
	}

	for i, part := range parts {
		lower := strings.ToLower(part)
		if acronym, ok := acronyms[lower]; ok {
			parts[i] = acronym
		} else {
			parts[i] = strings.Title(lower)
		}
	}

	return strings.Join(parts, " ")
}
