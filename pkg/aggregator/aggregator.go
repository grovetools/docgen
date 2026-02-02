package aggregator

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/docgen/pkg/capture"
	docgenConfig "github.com/grovetools/docgen/pkg/config"
	"github.com/grovetools/docgen/pkg/manifest"
	"github.com/grovetools/docgen/pkg/transformer"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Aggregator struct {
	logger *logrus.Logger
}

func New(logger *logrus.Logger) *Aggregator {
	return &Aggregator{logger: logger}
}

// Aggregate collects documentation from ecosystems specified in the local docgen.config.yml.
// If no ecosystems are specified, it falls back to the current ecosystem only and warns the user.
// The transform parameter specifies output transformations (e.g., "astro" for website builds).
func (a *Aggregator) Aggregate(outputDir string, mode string, transform string) error {
	// Validate mode
	if mode != "dev" && mode != "prod" {
		return fmt.Errorf("invalid mode '%s': must be 'dev' or 'prod'", mode)
	}

	a.logger.Infof("Aggregating documentation in %s mode", mode)

	// Try to load local docgen.config.yml to get ecosystems list
	// Uses LoadWithNotebook to check notebook location first, then repo
	cwd, _ := os.Getwd()
	localCfg, _, _ := docgenConfig.LoadWithNotebook(cwd)

	var ecosystemsToProcess []workspace.Ecosystem

	if localCfg != nil && len(localCfg.Settings.Ecosystems) > 0 {
		// Use explicitly configured ecosystems
		a.logger.Infof("Using ecosystems from local config: %v", localCfg.Settings.Ecosystems)

		// Discover all ecosystems to match by name
		discoveryService := workspace.NewDiscoveryService(a.logger)
		result, err := discoveryService.DiscoverAll()
		if err != nil {
			return fmt.Errorf("could not discover ecosystems: %w", err)
		}

		// Build a map for lookup
		ecoByName := make(map[string]workspace.Ecosystem)
		for _, eco := range result.Ecosystems {
			ecoByName[eco.Name] = eco
		}

		// Filter to only requested ecosystems
		for _, name := range localCfg.Settings.Ecosystems {
			if eco, ok := ecoByName[name]; ok {
				ecosystemsToProcess = append(ecosystemsToProcess, eco)
			} else {
				a.logger.Warnf("Ecosystem '%s' not found in groves config", name)
			}
		}

		if len(ecosystemsToProcess) == 0 {
			return fmt.Errorf("none of the specified ecosystems were found: %v", localCfg.Settings.Ecosystems)
		}
	} else {
		// No ecosystems specified - fall back to current ecosystem only
		rootDir, err := workspace.FindEcosystemRoot("")
		if err != nil {
			return fmt.Errorf("could not find ecosystem root: %w", err)
		}

		a.logger.Warnf("No 'ecosystems' specified in docgen.config.yml - using current ecosystem only (%s)", filepath.Base(rootDir))
		a.logger.Warnf("To aggregate from multiple ecosystems, add 'settings.ecosystems' to your docgen.config.yml")

		ecosystemsToProcess = []workspace.Ecosystem{{Name: filepath.Base(rootDir), Path: rootDir}}
	}

	a.logger.Infof("Processing %d ecosystem(s)", len(ecosystemsToProcess))

	// Build set of allowed packages from sidebar config
	// Only packages listed in sidebar.categories.*.packages will be aggregated
	allowedPackages := make(map[string]bool)
	if localCfg != nil && localCfg.Sidebar != nil && localCfg.Sidebar.Categories != nil {
		for _, cat := range localCfg.Sidebar.Categories {
			for _, pkg := range cat.Packages {
				allowedPackages[pkg] = true
			}
		}
		a.logger.Infof("Filtering to %d allowed packages from sidebar config", len(allowedPackages))
	}

	m := &manifest.Manifest{
		Packages:        []manifest.PackageManifest{},
		WebsiteSections: []manifest.WebsiteSection{},
	}

	// Aggregate from each ecosystem
	for _, eco := range ecosystemsToProcess {
		a.logger.Infof("Processing ecosystem: %s (%s)", eco.Name, eco.Path)
		if err := a.aggregateEcosystem(eco.Path, m, outputDir, mode, transform, allowedPackages); err != nil {
			a.logger.Warnf("Error aggregating ecosystem %s: %v", eco.Name, err)
			// Continue with other ecosystems
		}
	}

	// Include sidebar configuration if present in local config
	if localCfg != nil && localCfg.Sidebar != nil {
		m.Sidebar = a.buildSidebarManifest(localCfg.Sidebar, mode)
		a.logger.Infof("Including sidebar configuration from local config")
	}

	m.GeneratedAt = time.Now()

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save the manifest
	manifestPath := filepath.Join(outputDir, "manifest.json")
	a.logger.Infof("Saving manifest with %d packages and %d website sections", len(m.Packages), len(m.WebsiteSections))
	return m.Save(manifestPath)
}

// buildSidebarManifest creates the manifest sidebar config from the source config,
// filtering packages by status based on the build mode.
func (a *Aggregator) buildSidebarManifest(src *docgenConfig.SidebarConfig, mode string) *manifest.SidebarConfig {
	result := &manifest.SidebarConfig{
		CategoryOrder:           src.CategoryOrder,
		PackageCategoryOverride: src.PackageCategoryOverride,
	}

	// Copy categories with their config
	if src.Categories != nil {
		result.Categories = make(map[string]manifest.SidebarCategory)
		for name, cat := range src.Categories {
			result.Categories[name] = manifest.SidebarCategory{
				Icon:     cat.Icon,
				Flat:     cat.Flat,
				Packages: cat.Packages,
			}
		}
	}

	// Copy packages, filtering by status
	if src.Packages != nil {
		result.Packages = make(map[string]manifest.SidebarPackage)
		for name, pkg := range src.Packages {
			status := pkg.Status
			if status == "" {
				status = docgenConfig.StatusProduction // Default to production
			}

			// Filter by status:
			// - draft: excluded from all builds
			// - dev: included in dev mode only
			// - production: included in all builds
			if status == docgenConfig.StatusDraft {
				a.logger.Debugf("Excluding package %s from sidebar (status: draft)", name)
				continue
			}
			if mode == "prod" && status == docgenConfig.StatusDev {
				a.logger.Debugf("Excluding package %s from sidebar (status: dev, mode: prod)", name)
				continue
			}

			result.Packages[name] = manifest.SidebarPackage{
				Icon:   pkg.Icon,
				Color:  pkg.Color,
				Status: status,
			}
		}
	}

	return result
}

// aggregateEcosystem processes a single ecosystem and adds its docs to the manifest
// If allowedPackages is non-empty, only packages in that set will be included.
// The transform parameter specifies output transformations (e.g., "astro" for website builds).
func (a *Aggregator) aggregateEcosystem(rootDir string, m *manifest.Manifest, outputDir, mode, transform string, allowedPackages map[string]bool) error {
	// Load the ecosystem config to get workspace paths
	configPath, err := config.FindConfigFile(rootDir)
	if err != nil {
		return fmt.Errorf("could not find config file in %s: %w", rootDir, err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("could not load ecosystem config from %s: %w", configPath, err)
	}

	a.logger.Debugf("Loaded ecosystem config with %d workspace patterns", len(cfg.Workspaces))

	// Get workspace paths from config (expand glob patterns)
	var workspaces []string
	for _, wsPattern := range cfg.Workspaces {
		pattern := filepath.Join(rootDir, wsPattern)

		matches, err := filepath.Glob(pattern)
		if err != nil {
			a.logger.Warnf("Failed to expand pattern %s: %v", wsPattern, err)
			continue
		}

		for _, match := range matches {
			// Only include directories
			if info, err := os.Stat(match); err == nil && info.IsDir() {
				a.logger.Debugf("  Found workspace: %s", match)
				workspaces = append(workspaces, match)
			}
		}
	}

	a.logger.Debugf("Total workspaces in ecosystem: %d", len(workspaces))

	for _, wsPath := range workspaces {
		wsName := filepath.Base(wsPath)
		docCfg, err := docgenConfig.Load(wsPath)
		if err != nil {
			if os.IsNotExist(err) {
				a.logger.Debugf("Skipping %s: no docgen.config.yml found", wsName)
			} else {
				a.logger.Warnf("Skipping %s: could not load config: %v", wsName, err)
			}
			continue
		}

		if !docCfg.Enabled {
			a.logger.Infof("Skipping %s: documentation is disabled in config", wsName)
			continue
		}

		// Skip packages not in the allowed set (if filtering is enabled)
		if len(allowedPackages) > 0 && !allowedPackages[wsName] {
			// Also check if this is a "sections" mode config (website content) - always allow those
			if docCfg.Settings.OutputMode != "sections" {
				a.logger.Debugf("Skipping %s: not in allowed packages list", wsName)
				continue
			}
		}

		// Handle "sections" output mode (for website content like overview, concepts)
		if docCfg.Settings.OutputMode == "sections" {
			a.processWebsiteSections(wsPath, docCfg, m, outputDir, mode, transform)
			continue
		}

		// Get version and repo URL
		version := a.getPackageVersion(wsPath)
		repoURL := a.getRepoURL(wsPath)

		// Add to manifest
		pkgManifest := manifest.PackageManifest{
			Name:        wsName,
			Title:       docCfg.Title,
			Description: docCfg.Description,
			Category:    docCfg.Category,
			DocsPath:    fmt.Sprintf("./%s", wsName),
			Version:     version,
			RepoURL:     repoURL,
		}

		// Resolve docs directory (notebook or repo)
		docsDir := a.resolveDocsDirForWorkspace(wsPath)

		// Filter sections based on status:
		// - draft: excluded from all builds
		// - dev: included in dev mode only
		// - production: included in all builds
		var sectionsToAggregate []docgenConfig.SectionConfig
		for _, section := range docCfg.Sections {
			status := section.GetStatus()

			if status == docgenConfig.StatusDraft {
				a.logger.Debugf("Skipping %s/%s (status: draft)", wsName, section.Output)
				continue
			}
			if mode == "prod" && status == docgenConfig.StatusDev {
				a.logger.Debugf("Skipping %s/%s (status: dev, mode: prod)", wsName, section.Output)
				continue
			}

			sectionsToAggregate = append(sectionsToAggregate, section)
		}

		// Skip this package entirely if no sections are available after filtering
		if len(sectionsToAggregate) == 0 {
			a.logger.Infof("Skipping package %s: no sections available in %s mode", wsName, mode)
			continue
		}

		// Copy generated files and build section manifest
		// Copy only the markdown output files specified in the config, not everything in docs/
		// Create output directory only if we have sections to copy
		distDest := filepath.Join(outputDir, wsName)
		if err := os.MkdirAll(distDest, 0755); err != nil {
			a.logger.WithError(err).Errorf("Failed to create output directory for %s", wsName)
			continue
		}

		for _, section := range sectionsToAggregate {
			srcFile := filepath.Join(docsDir, section.Output)
			destFile := filepath.Join(distDest, section.Output)

			// Handle capture sections - generate on-the-fly during aggregation
			if section.Type == "capture" {
				if section.Binary == "" {
					a.logger.Warnf("Capture section %s/%s missing 'binary' field, skipping", wsName, section.Name)
					continue
				}

				a.logger.Infof("Capturing CLI reference for %s/%s (binary: %s)", wsName, section.Name, section.Binary)

				// Determine format and depth
				format := capture.FormatHTML
				if section.Format == "plain" {
					format = capture.FormatMarkdown
				}
				depth := 5
				if section.Depth > 0 {
					depth = section.Depth
				}

				// Run capture directly to destination
				capturer := capture.New(a.logger)
				opts := capture.Options{
					MaxDepth:        depth,
					Format:          format,
					SubcommandOrder: section.SubcommandOrder,
				}

				if err := capturer.Capture(section.Binary, destFile, opts); err != nil {
					a.logger.WithError(err).Errorf("Failed to capture CLI for %s/%s", wsName, section.Name)
					continue
				}

				// Apply Astro transformations if requested
				if transform == "astro" {
					srcData, err := os.ReadFile(destFile)
					if err != nil {
						a.logger.WithError(err).Errorf("Failed to read captured file %s", destFile)
						continue
					}

					trans := transformer.NewAstroTransformer()
					opts := transformer.TransformOptions{
						PackageName: wsName,
						Title:       section.Title,
						Description: docCfg.Description,
						Version:     version,
						Category:    docCfg.Category,
						Order:       section.Order,
					}
					processedData := trans.TransformStandardDoc(srcData, opts)

					if err := os.WriteFile(destFile, processedData, 0644); err != nil {
						a.logger.WithError(err).Errorf("Failed to write transformed %s", destFile)
						continue
					}
				}

				continue
			}

			// Check if the actual documentation file exists
			if _, err := os.Stat(srcFile); os.IsNotExist(err) {
				// Try to use the prompt file as a placeholder
				// Resolve prompt using notebook location first
				promptData, promptErr := a.resolvePromptForWorkspace(wsPath, section.Prompt)
				if promptErr == nil {
					a.logger.Infof("Using prompt file as placeholder for %s/%s", wsName, section.Output)

					// Add a header to indicate this is a placeholder
					placeholder := fmt.Sprintf("# %s\n\n*Note: This is a placeholder generated from the prompt file. Full documentation is pending.*\n\n---\n\n%s", section.Title, string(promptData))

					if err := os.WriteFile(destFile, []byte(placeholder), 0644); err != nil {
						a.logger.WithError(err).Errorf("Failed to write placeholder %s", destFile)
						continue
					}
				} else {
					a.logger.Warnf("No documentation or prompt found for %s/%s: %v", wsName, section.Output, promptErr)
					continue
				}
			} else {
				// Copy the actual documentation file
				a.logger.Infof("Copying documentation for %s/%s", wsName, section.Output)

				srcData, err := os.ReadFile(srcFile)
				if err != nil {
					a.logger.WithError(err).Errorf("Failed to read %s", srcFile)
					continue
				}

				// Apply agg_strip_lines if configured for this section
				processedData := a.applyStripLines(srcData, section.AggStripLines, wsName, section.Output)

				// Apply Astro transformations if requested
				if transform == "astro" {
					trans := transformer.NewAstroTransformer()
					opts := transformer.TransformOptions{
						PackageName: wsName,
						Title:       section.Title,
						Description: docCfg.Description,
						Version:     version,
						Category:    docCfg.Category,
						Order:       section.Order,
					}
					processedData = trans.TransformStandardDoc(processedData, opts)
				}

				if err := os.WriteFile(destFile, processedData, 0644); err != nil {
					a.logger.WithError(err).Errorf("Failed to write %s", destFile)
					continue
				}
			}
		}

		// Copy images directory - try notebook location first, then docs/
		imagesSrcPath := a.resolveAssetsDirForWorkspace(wsPath, "images")
		if imagesSrcPath != "" {
			imagesDestPath := filepath.Join(distDest, "images")
			a.logger.Infof("Copying images for %s from %s to %s", wsName, imagesSrcPath, imagesDestPath)
			if err := copyDir(imagesSrcPath, imagesDestPath); err != nil {
				a.logger.WithError(err).Errorf("Failed to copy images directory for %s", wsName)
				// Log error but continue
			}
		}

		// Copy asciicasts directory - try notebook location first, then docs/
		asciicastsSrcPath := a.resolveAssetsDirForWorkspace(wsPath, "asciicasts")
		if asciicastsSrcPath != "" {
			asciicastsDestPath := filepath.Join(distDest, "asciicasts")
			a.logger.Infof("Copying asciicasts for %s from %s to %s", wsName, asciicastsSrcPath, asciicastsDestPath)
			if err := copyDir(asciicastsSrcPath, asciicastsDestPath); err != nil {
				a.logger.WithError(err).Errorf("Failed to copy asciicasts directory for %s", wsName)
				// Log error but continue
			}
		}

		// Copy videos directory - try notebook location first, then docs/
		videosSrcPath := a.resolveAssetsDirForWorkspace(wsPath, "videos")
		if videosSrcPath != "" {
			videosDestPath := filepath.Join(distDest, "videos")
			a.logger.Infof("Copying videos for %s from %s to %s", wsName, videosSrcPath, videosDestPath)
			if err := copyDir(videosSrcPath, videosDestPath); err != nil {
				a.logger.WithError(err).Errorf("Failed to copy videos directory for %s", wsName)
				// Log error but continue
			}
		}

		// Copy additional logo files specified in logos: config
		if len(docCfg.Logos) > 0 {
			imagesDestPath := filepath.Join(distDest, "images")
			if err := os.MkdirAll(imagesDestPath, 0755); err != nil {
				a.logger.WithError(err).Errorf("Failed to create images directory for logos: %s", wsName)
			} else {
				for _, logoPath := range docCfg.Logos {
					// Expand ~ in path
					expandedPath := expandPath(logoPath)
					if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
						a.logger.Warnf("Logo file not found for %s: %s", wsName, expandedPath)
						continue
					}
					logoDestPath := filepath.Join(imagesDestPath, filepath.Base(expandedPath))
					a.logger.Infof("Copying logo for %s: %s -> %s", wsName, expandedPath, logoDestPath)
					if err := copyFile(expandedPath, logoDestPath); err != nil {
						a.logger.WithError(err).Errorf("Failed to copy logo file for %s", wsName)
					}
				}
			}
		}

		// Aggregate concepts from the workspace's concepts directory
		if err := a.aggregateConcepts(wsPath, wsName, docCfg, distDest, mode, transform); err != nil {
			a.logger.Warnf("Failed to aggregate concepts for %s: %v", wsName, err)
		}

		sort.Slice(sectionsToAggregate, func(i, j int) bool {
			return sectionsToAggregate[i].Order < sectionsToAggregate[j].Order
		})

		for _, sec := range sectionsToAggregate {
			pkgManifest.Sections = append(pkgManifest.Sections, manifest.SectionManifest{
				Title: sec.Title,
				Path:  fmt.Sprintf("./%s/%s", wsName, sec.Output),
			})
		}

		// Check for and copy CHANGELOG.md if it exists
		changelogSrc := filepath.Join(wsPath, "CHANGELOG.md")
		if _, err := os.Stat(changelogSrc); err == nil {
			changelogDest := filepath.Join(distDest, "CHANGELOG.md")

			// Copy the CHANGELOG.md file
			changelogData, err := os.ReadFile(changelogSrc)
			if err != nil {
				a.logger.WithError(err).Errorf("Failed to read CHANGELOG.md for %s", wsName)
			} else {
				// Apply Astro transformations if requested
				if transform == "astro" {
					trans := transformer.NewAstroTransformer()
					opts := transformer.TransformOptions{
						PackageName: wsName,
						Title:       fmt.Sprintf("Changelog for %s", docCfg.Title),
						Description: "",
						Version:     version,
						Category:    docCfg.Category,
						Order:       999, // Changelogs go at the end
					}
					changelogData = trans.TransformStandardDoc(changelogData, opts)
				}

				if err := os.WriteFile(changelogDest, changelogData, 0644); err != nil {
					a.logger.WithError(err).Errorf("Failed to write CHANGELOG.md for %s", wsName)
				} else {
					// Update the manifest with the changelog path
					pkgManifest.ChangelogPath = fmt.Sprintf("./%s/CHANGELOG.md", wsName)
					a.logger.Infof("Copied CHANGELOG.md for %s", wsName)
				}
			}
		} else {
			a.logger.Debugf("No CHANGELOG.md found for %s", wsName)
		}

		m.Packages = append(m.Packages, pkgManifest)
	}

	return nil
}

// getPackageVersion attempts to get the version from git tags or grove.yml
func (a *Aggregator) getPackageVersion(wsPath string) string {
	// Try to get version from git tags
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = wsPath
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		if version != "" {
			return version
		}
	}

	// Fall back to checking grove config for version info
	if configPath, err := config.FindConfigFile(wsPath); err == nil {
		if cfg, err := config.Load(configPath); err == nil && cfg.Version != "" {
			return cfg.Version
		}
	}

	// Default to "latest" if no version found
	return "latest"
}

// getRepoURL attempts to get the repository URL from git remote
func (a *Aggregator) getRepoURL(wsPath string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = wsPath
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(output))
	// Convert SSH URLs to HTTPS URLs for consistency
	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	}
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")
	
	return url
}

// resolveDocsDirForWorkspace finds the docs directory for a given workspace,
// trying notebook location first, then falling back to repo docs/ directory.
// Returns the path to the docs directory to use for reading documentation files.
func (a *Aggregator) resolveDocsDirForWorkspace(wsPath string) string {
	// 1. Try to get workspace node
	node, err := workspace.GetProjectByPath(wsPath)
	if err != nil {
		// Fallback: Can't resolve workspace, use repo path
		a.logger.Debugf("Could not resolve workspace for %s, using repo docs/ path", wsPath)
		return filepath.Join(wsPath, "docs")
	}

	// 2. Try notebook path first
	cfg, err := config.LoadDefault()
	if err == nil {
		locator := workspace.NewNotebookLocator(cfg)
		docgenDir, err := locator.GetDocgenDir(node)

		if err == nil {
			notebookDocsPath := filepath.Join(docgenDir, "docs")
			if _, err := os.Stat(notebookDocsPath); err == nil {
				a.logger.Debugf("Found docs directory in notebook: %s", notebookDocsPath)
				return notebookDocsPath
			}
		}
	}

	// 3. Fallback to repo docs/ path
	repoDocsPath := filepath.Join(wsPath, "docs")
	a.logger.Debugf("Using repo docs directory: %s", repoDocsPath)
	return repoDocsPath
}

// resolveAssetsDirForWorkspace finds the assets directory for a given workspace,
// trying notebook location first, then falling back to docs/ directory.
// Returns empty string if the directory doesn't exist in either location.
func (a *Aggregator) resolveAssetsDirForWorkspace(wsPath, assetType string) string {
	// 1. Try to get workspace node
	node, err := workspace.GetProjectByPath(wsPath)
	if err != nil {
		// Fallback: Can't resolve workspace, use legacy path
		a.logger.Debugf("Could not resolve workspace for %s, trying docs/ path", wsPath)
		legacyPath := filepath.Join(wsPath, "docs", assetType)
		if _, err := os.Stat(legacyPath); err == nil {
			return legacyPath
		}
		return ""
	}

	// 2. Try notebook path first
	cfg, err := config.LoadDefault()
	if err == nil {
		locator := workspace.NewNotebookLocator(cfg)
		promptsDir, err := locator.GetDocgenPromptsDir(node)

		if err == nil {
			// GetDocgenPromptsDir returns the prompts subdirectory (docgen/prompts)
			// Assets live at the parent level (docgen/images, docgen/asciicasts, etc.)
			docgenDir := filepath.Dir(promptsDir)
			notebookPath := filepath.Join(docgenDir, assetType)
			if _, err := os.Stat(notebookPath); err == nil {
				a.logger.Debugf("Found %s directory in notebook: %s", assetType, notebookPath)
				return notebookPath
			}
		}
	}

	// 3. Fallback to legacy docs/ path
	legacyPath := filepath.Join(wsPath, "docs", assetType)
	if _, err := os.Stat(legacyPath); err == nil {
		a.logger.Debugf("Found %s directory in docs/: %s", assetType, legacyPath)
		return legacyPath
	}

	return ""
}

// resolvePromptForWorkspace finds and reads a prompt file for a given workspace,
// trying notebook location first, then falling back to legacy location.
func (a *Aggregator) resolvePromptForWorkspace(wsPath, promptFile string) ([]byte, error) {
	// Extract basename only for backward compatibility
	promptBaseName := filepath.Base(promptFile)

	// 1. Try to get workspace node
	node, err := workspace.GetProjectByPath(wsPath)
	if err != nil {
		// Fallback: Can't resolve workspace, use legacy path
		a.logger.Debugf("Could not resolve workspace for %s, trying legacy path", wsPath)
		legacyPath := filepath.Join(wsPath, "docs", "prompts", promptFile)
		return os.ReadFile(legacyPath)
	}

	// 2. Try notebook path first
	cfg, err := config.LoadDefault()
	if err == nil {
		locator := workspace.NewNotebookLocator(cfg)
		notebookPromptsDir, err := locator.GetDocgenPromptsDir(node)

		if err == nil {
			notebookPath := filepath.Join(notebookPromptsDir, promptBaseName)
			if data, err := os.ReadFile(notebookPath); err == nil {
				a.logger.Debugf("Loaded prompt '%s' from notebook: %s", promptBaseName, notebookPath)
				return data, nil
			}
		}
	}

	// 3. Fallback to legacy path
	legacyPath := filepath.Join(wsPath, "docs", "prompts", promptFile)
	a.logger.Debugf("Prompt not found in notebook, trying legacy path: %s", legacyPath)
	return os.ReadFile(legacyPath)
}

// applyStripLines removes specified number of lines from the beginning of content during aggregation
func (a *Aggregator) applyStripLines(content []byte, aggStripLines int, packageName, sectionOutput string) []byte {
	if aggStripLines <= 0 {
		return content
	}
	
	lines := strings.Split(string(content), "\n")
	if len(lines) > aggStripLines {
		// Join the remaining lines after stripping
		stripped := strings.Join(lines[aggStripLines:], "\n")
		a.logger.Debugf("Stripped %d lines from %s/%s during aggregation", aggStripLines, packageName, sectionOutput)
		return []byte(stripped)
	} else {
		// If file has fewer lines than agg_strip_lines, result is empty
		a.logger.Warnf("Source file %s/%s has fewer lines (%d) than agg_strip_lines setting (%d)", 
			packageName, sectionOutput, len(lines), aggStripLines)
		return []byte("")
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// processWebsiteSections handles documentation with output_mode: sections
// This is used for website content that maps to multiple Astro content collections
// (e.g., overview/, concepts/) rather than a single package directory.
//
// Each section subdirectory should have its own docgen.config.yml (like a mini-package),
// mirroring the structure of package docgen directories (docs/, prompts/, images/, etc.)
func (a *Aggregator) processWebsiteSections(wsPath string, cfg *docgenConfig.DocgenConfig, m *manifest.Manifest, outputDir, mode, transform string) {
	wsName := filepath.Base(wsPath)
	a.logger.Infof("Processing website sections for %s", wsName)

	// Resolve base docgen directory (notebook first, then repo)
	node, err := workspace.GetProjectByPath(wsPath)
	if err != nil {
		a.logger.Warnf("Could not resolve workspace for %s, skipping sections: %v", wsPath, err)
		return
	}

	// Determine base path for docs - try notebook first
	var baseDocgenDir string
	coreCfg, err := config.LoadDefault()
	if err == nil {
		locator := workspace.NewNotebookLocator(coreCfg)
		if dir, err := locator.GetDocgenDir(node); err == nil && dirExists(dir) {
			baseDocgenDir = dir
			a.logger.Debugf("Using notebook docgen dir: %s", baseDocgenDir)
		}
	}

	if baseDocgenDir == "" {
		// Fallback to repo docs/ directory
		baseDocgenDir = filepath.Join(wsPath, "docs")
		a.logger.Debugf("Falling back to repo docs dir: %s", baseDocgenDir)
	}

	// Discover section subdirectories that have their own docgen.config.yml
	entries, err := os.ReadDir(baseDocgenDir)
	if err != nil {
		a.logger.Warnf("Failed to read docgen dir %s: %v", baseDocgenDir, err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sectionName := entry.Name()
		sectionDir := filepath.Join(baseDocgenDir, sectionName)

		// Check if this subdirectory has its own docgen.config.yml
		sectionConfigPath := filepath.Join(sectionDir, docgenConfig.ConfigFileName)
		if _, err := os.Stat(sectionConfigPath); os.IsNotExist(err) {
			continue // Not a section directory
		}

		// Load the section's config
		sectionCfg, err := docgenConfig.LoadFromPath(sectionConfigPath)
		if err != nil {
			a.logger.Warnf("Failed to load config for section %s: %v", sectionName, err)
			continue
		}

		if !sectionCfg.Enabled {
			a.logger.Debugf("Skipping section %s: disabled in config", sectionName)
			continue
		}

		a.logger.Infof("Processing section: %s (%s)", sectionName, sectionCfg.Title)

		// Create output directory for this section
		destDir := filepath.Join(outputDir, sectionName)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			a.logger.Errorf("Failed to create dest dir %s: %v", destDir, err)
			continue
		}

		websiteSection := manifest.WebsiteSection{
			Name:  sectionName,
			Title: sectionCfg.Title,
			Files: []manifest.SectionManifest{},
		}

		// Copy assets from section directory
		for _, assetType := range []string{"images", "asciicasts", "videos"} {
			assetSrc := filepath.Join(sectionDir, assetType)
			if dirExists(assetSrc) {
				assetDest := filepath.Join(destDir, assetType)
				if err := copyDir(assetSrc, assetDest); err != nil {
					a.logger.Warnf("Failed to copy %s for section %s: %v", assetType, sectionName, err)
				} else {
					a.logger.Debugf("Copied %s for section %s", assetType, sectionName)
				}
			}
		}

		// Resolve docs directory (respects settings.output_dir from section config)
		docsSubdir := "docs"
		if sectionCfg.Settings.OutputDir != "" {
			docsSubdir = sectionCfg.Settings.OutputDir
		}
		docsDir := filepath.Join(sectionDir, docsSubdir)

		// Process sections from the section's config (like a mini-package)
		for _, sec := range sectionCfg.Sections {
			status := sec.GetStatus()

			// Filter by status
			if status == docgenConfig.StatusDraft {
				a.logger.Debugf("Skipping %s/%s (status: draft)", sectionName, sec.Output)
				continue
			}
			if mode == "prod" && status == docgenConfig.StatusDev {
				a.logger.Debugf("Skipping %s/%s (status: dev, mode: prod)", sectionName, sec.Output)
				continue
			}

			srcFile := filepath.Join(docsDir, sec.Output)
			if _, err := os.Stat(srcFile); os.IsNotExist(err) {
				a.logger.Warnf("Doc file not found: %s", srcFile)
				continue
			}

			// Read file content
			content, err := os.ReadFile(srcFile)
			if err != nil {
				a.logger.Warnf("Failed to read %s: %v", sec.Output, err)
				continue
			}

			// Apply Astro transformations if requested
			if transform == "astro" {
				trans := transformer.NewAstroTransformer()
				opts := transformer.TransformOptions{
					SectionName: sectionName,
					Category:    sectionCfg.Category,
				}
				content = trans.TransformWebsiteSection(content, opts)
			}

			// Write file
			destPath := filepath.Join(destDir, sec.Output)
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				a.logger.Warnf("Failed to write %s: %v", sec.Output, err)
				continue
			}

			websiteSection.Files = append(websiteSection.Files, manifest.SectionManifest{
				Name:  sec.Output,
				Title: sec.Title,
				Order: sec.Order,
				Path:  fmt.Sprintf("./%s/%s", sectionName, sec.Output),
			})
		}

		// Sort files by order
		sort.Slice(websiteSection.Files, func(i, j int) bool {
			return websiteSection.Files[i].Order < websiteSection.Files[j].Order
		})

		if len(websiteSection.Files) > 0 {
			m.WebsiteSections = append(m.WebsiteSections, websiteSection)
			a.logger.Infof("Added website section %s with %d files", sectionName, len(websiteSection.Files))
		}
	}
}

// ConceptManifest represents the concept-manifest.yml structure
type ConceptManifest struct {
	ID            string   `yaml:"id"`
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	Status        string   `yaml:"status"`
	DocgenPublish string   `yaml:"docgen_publish"` // draft | dev | production
	DocgenOrder   []string `yaml:"docgen_order"`   // ordered list of .md files
}

// aggregateConcepts scans the workspace's concepts directory and copies publishable concepts
func (a *Aggregator) aggregateConcepts(wsPath string, wsName string, docCfg *docgenConfig.DocgenConfig, distDest string, mode string, transform string) error {
	// 1. Find concepts directory via notebook locator
	node, err := workspace.GetProjectByPath(wsPath)
	if err != nil {
		a.logger.Debugf("Could not resolve workspace for concepts: %v", err)
		return nil // Not an error, just skip concepts
	}

	coreCfg, err := config.LoadDefault()
	if err != nil {
		return nil
	}

	locator := workspace.NewNotebookLocator(coreCfg)
	docgenDir, err := locator.GetDocgenDir(node)
	if err != nil {
		return nil
	}

	// concepts is at the same level as docgen: {notebook}/workspaces/{name}/concepts/
	workspaceDir := filepath.Dir(docgenDir)
	conceptsDir := filepath.Join(workspaceDir, "concepts")

	if !dirExists(conceptsDir) {
		a.logger.Debugf("No concepts directory found for %s", wsName)
		return nil
	}

	// 2. Scan for concept subdirectories
	entries, err := os.ReadDir(conceptsDir)
	if err != nil {
		return fmt.Errorf("failed to read concepts directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		conceptID := entry.Name()
		conceptDir := filepath.Join(conceptsDir, conceptID)

		// 3. Read concept manifest
		manifestPath := filepath.Join(conceptDir, "concept-manifest.yml")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			a.logger.Debugf("No manifest for concept %s: %v", conceptID, err)
			continue
		}

		var cm ConceptManifest
		if err := yaml.Unmarshal(manifestData, &cm); err != nil {
			a.logger.Warnf("Failed to parse manifest for concept %s: %v", conceptID, err)
			continue
		}

		// 4. Check docgen_publish status
		publishStatus := cm.DocgenPublish
		if publishStatus == "" {
			publishStatus = docgenConfig.StatusDraft // Default to draft (not published)
		}

		if publishStatus == docgenConfig.StatusDraft {
			a.logger.Debugf("Skipping concept %s (docgen_publish: draft)", conceptID)
			continue
		}
		if mode == "prod" && publishStatus == docgenConfig.StatusDev {
			a.logger.Debugf("Skipping concept %s (docgen_publish: dev, mode: prod)", conceptID)
			continue
		}

		a.logger.Infof("Aggregating concept: %s/%s", wsName, conceptID)

		// 5. Find all .md files in the concept, respecting docgen_order
		var mdFiles []string
		if len(cm.DocgenOrder) > 0 {
			// Use explicit order from manifest
			for _, f := range cm.DocgenOrder {
				path := filepath.Join(conceptDir, f)
				if _, err := os.Stat(path); err == nil {
					mdFiles = append(mdFiles, path)
				}
			}
		} else {
			// Fall back to glob (alphabetical)
			mdFiles, _ = filepath.Glob(filepath.Join(conceptDir, "*.md"))
		}
		if len(mdFiles) == 0 {
			a.logger.Debugf("No markdown files in concept %s", conceptID)
			continue
		}

		// 6. Create output directory: {dist}/{pkg}/concepts/{concept-id}/
		conceptDestDir := filepath.Join(distDest, "concepts", conceptID)
		if err := os.MkdirAll(conceptDestDir, 0755); err != nil {
			a.logger.Errorf("Failed to create concept output dir: %v", err)
			continue
		}

		// 7. Copy each .md file with proper frontmatter
		for i, mdPath := range mdFiles {
			mdFile := filepath.Base(mdPath)
			content, err := os.ReadFile(mdPath)
			if err != nil {
				a.logger.Warnf("Failed to read %s: %v", mdPath, err)
				continue
			}

			// Strip existing frontmatter
			body := stripMarkdownFrontmatter(string(content))

			// Generate title from filename
			title := formatConceptTitle(strings.TrimSuffix(mdFile, ".md"))

			// Calculate order: concepts start at high numbers to appear after regular docs
			order := 2000 + i + 1

			// Build new content with Astro-compatible frontmatter
			var newContent string
			if transform == "astro" {
				newContent = fmt.Sprintf(`---
title: "%s"
package: "%s"
category: "%s"
order: %d
concept_title: "%s"
concept_id: "%s"
---

%s`, title, wsName, docCfg.Category, order, cm.Title, conceptID, body)
			} else {
				newContent = string(content)
			}

			// Write to destination
			destPath := filepath.Join(conceptDestDir, mdFile)
			if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
				a.logger.Errorf("Failed to write %s: %v", destPath, err)
				continue
			}

			a.logger.Debugf("Copied concept doc: %s", destPath)
		}
	}

	return nil
}

// stripMarkdownFrontmatter removes YAML frontmatter from markdown content
func stripMarkdownFrontmatter(content string) string {
	re := regexp.MustCompile(`(?s)^---\n.*?\n---\n*`)
	return re.ReplaceAllString(content, "")
}

// formatConceptTitle converts a filename to a title
// e.g., "cli-output-destinations" -> "CLI Output Destinations"
func formatConceptTitle(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	})

	acronyms := map[string]string{
		"cli": "CLI", "tui": "TUI", "api": "API",
		"ui": "UI", "id": "ID", "llm": "LLM",
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

// parseFrontmatter extracts status, title, and order from markdown frontmatter
func parseFrontmatter(path string) (status, title string, order int) {
	status = docgenConfig.StatusProduction // Default
	title = ""
	order = 0

	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	s := string(content)
	if !strings.HasPrefix(s, "---\n") {
		// No frontmatter, try to extract order from filename (e.g., "01-intro.md" -> 1)
		order = extractOrderFromFilename(filepath.Base(path))
		return
	}

	end := strings.Index(s[4:], "\n---")
	if end == -1 {
		return
	}

	frontmatter := s[4 : end+4]
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "status:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				status = strings.TrimSpace(parts[1])
				// Remove quotes if present
				status = strings.Trim(status, "\"'")
			}
		} else if strings.HasPrefix(line, "title:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				title = strings.TrimSpace(parts[1])
				title = strings.Trim(title, "\"'")
			}
		} else if strings.HasPrefix(line, "order:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &order)
			}
		}
	}

	// If no order in frontmatter, try filename
	if order == 0 {
		order = extractOrderFromFilename(filepath.Base(path))
	}

	return
}

// extractOrderFromFilename extracts order from filenames like "01-intro.md" -> 1
func extractOrderFromFilename(filename string) int {
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Try to parse leading number
	var order int
	if _, err := fmt.Sscanf(name, "%d-", &order); err == nil {
		return order
	}
	return 0
}

// copyFile copies a single file from src to dst
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

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// expandPath expands ~ to user home directory
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}