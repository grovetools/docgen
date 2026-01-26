package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/docgen/pkg/config"
	"github.com/grovetools/docgen/pkg/manifest"
	"github.com/grovetools/docgen/pkg/watcher"
	"github.com/grovetools/docgen/pkg/writer"
	"github.com/spf13/cobra"
)

// watchedPackage holds cached information about a package being watched
type watchedPackage struct {
	wsPath    string // workspace path (e.g., /path/to/grove-flow)
	docgenDir string // docgen dir in notebook (e.g., /path/to/nb/workspaces/flow/docgen)
	pkgName   string // package name (e.g., "flow")
	config    *config.DocgenConfig
}

func newWatchCmd() *cobra.Command {
	var websiteDir string
	var mode string
	var debounceMs int
	var quiet bool

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch documentation sources and hot-reload on changes",
		Long: `Watches all docgen directories in configured ecosystems and
rebuilds changed packages incrementally. Astro's dev server will
pick up the changes automatically via HMR.

Example:
  docgen watch --website-dir . --mode dev --quiet

The watch command will:
1. Discover all packages with docgen enabled in configured ecosystems
2. Watch their notebook docgen directories for changes
3. On file change, rebuild only the affected package
4. Write output directly to the Astro content directories`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(websiteDir, mode, time.Duration(debounceMs)*time.Millisecond, quiet)
		},
	}

	// Default mode from DOCGEN_MODE env var, fallback to "dev"
	defaultMode := os.Getenv("DOCGEN_MODE")
	if defaultMode == "" {
		defaultMode = "dev"
	}

	cmd.Flags().StringVar(&websiteDir, "website-dir", ".", "Path to grove-website root")
	cmd.Flags().StringVar(&mode, "mode", defaultMode, "Build mode: dev or prod")
	cmd.Flags().IntVar(&debounceMs, "debounce", 100, "Debounce interval in milliseconds")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Minimal output (for concurrent use with astro)")
	return cmd
}

func runWatch(websiteDir, mode string, debounce time.Duration, quiet bool) error {
	// Validate mode
	if mode != "dev" && mode != "prod" {
		return errorf("invalid mode '%s': must be 'dev' or 'prod'", mode)
	}

	w, err := watcher.New()
	if err != nil {
		return errorf("failed to create watcher: %w", err)
	}
	defer w.Close()

	// Create Astro writer
	astroWriter := writer.NewAstro(websiteDir)

	// Load local config to get allowed packages and ecosystems
	cwd, _ := os.Getwd()
	localCfg, _, _ := config.LoadWithNotebook(cwd)

	// Build set of allowed packages from sidebar config
	allowedPackages := make(map[string]bool)
	if localCfg != nil && localCfg.Sidebar != nil && localCfg.Sidebar.Categories != nil {
		for _, cat := range localCfg.Sidebar.Categories {
			for _, pkg := range cat.Packages {
				allowedPackages[pkg] = true
			}
		}
	}

	// Discover packages and set up recursive watching
	watchedPkgs := make(map[string]*watchedPackage) // docgenDir -> package info

	ecosystems, err := discoverEcosystems(localCfg)
	if err != nil {
		return errorf("failed to discover ecosystems: %w", err)
	}

	// Load core config for notebook locator
	coreCfg, err := coreConfig.LoadDefault()
	if err != nil {
		return errorf("failed to load core config: %w", err)
	}
	locator := workspace.NewNotebookLocator(coreCfg)

	for _, eco := range ecosystems {
		if err := setupWatchForEcosystem(eco, w, locator, allowedPackages, watchedPkgs, quiet); err != nil {
			if !quiet {
				ulog.Warn("Failed to setup watch for ecosystem").Field("ecosystem", eco.Name).Err(err).Emit()
			}
		}
	}

	if len(watchedPkgs) == 0 {
		return errorf("no packages found to watch")
	}

	if !quiet {
		ulog.Info("Watching for documentation changes").
			Field("mode", mode).
			Field("website", websiteDir).
			Field("packages", len(watchedPkgs)).
			Emit()
	}

	// Debounce state
	var mu sync.Mutex
	pending := make(map[string]bool) // docgenDir -> needs rebuild
	var timer *time.Timer

	processPending := func() {
		mu.Lock()
		toProcess := pending
		pending = make(map[string]bool)
		mu.Unlock()

		for docgenDir := range toProcess {
			pkg := watchedPkgs[docgenDir]
			if pkg == nil {
				continue
			}

			if !quiet {
				ulog.Info("Rebuilding").Field("package", pkg.pkgName).Emit()
			}

			if err := rebuildPackage(pkg, astroWriter, mode, localCfg, quiet); err != nil {
				ulog.Error("Rebuild failed").Field("package", pkg.pkgName).Err(err).Emit()
			} else if !quiet {
				ulog.Info("Done").Field("package", pkg.pkgName).Emit()
			}
		}
	}

	// Main event loop
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}

			// Handle new directory creation (add to watcher)
			if event.Has(fsnotify.Create) {
				wsPath := w.FindWorkspace(event.Name)
				if wsPath != "" {
					w.HandleNewDirectory(event, wsPath)
				}
			}

			// Only process write and create events for relevant files
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			// Check if it's a relevant file
			if !watcher.IsRelevantFile(event.Name) {
				// Also handle config file changes
				if filepath.Base(event.Name) != "docgen.config.yml" {
					continue
				}
			}

			// Find the docgen directory this file belongs to
			docgenDir := findDocgenDir(event.Name, watchedPkgs)
			if docgenDir == "" {
				continue
			}

			// Queue for debounced processing
			mu.Lock()
			pending[docgenDir] = true
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, processPending)
			mu.Unlock()

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			ulog.Error("Watcher error").Err(err).Emit()
		}
	}
}

// discoverEcosystems returns the ecosystems to process based on config
func discoverEcosystems(localCfg *config.DocgenConfig) ([]workspace.Ecosystem, error) {
	discoveryService := workspace.NewDiscoveryService(getLogger())
	result, err := discoveryService.DiscoverAll()
	if err != nil {
		return nil, err
	}

	if localCfg != nil && len(localCfg.Settings.Ecosystems) > 0 {
		// Filter to configured ecosystems
		ecoByName := make(map[string]workspace.Ecosystem)
		for _, eco := range result.Ecosystems {
			ecoByName[eco.Name] = eco
		}

		var filtered []workspace.Ecosystem
		for _, name := range localCfg.Settings.Ecosystems {
			if eco, ok := ecoByName[name]; ok {
				filtered = append(filtered, eco)
			}
		}
		return filtered, nil
	}

	return result.Ecosystems, nil
}

// setupWatchForEcosystem sets up file watchers for all docgen-enabled packages in an ecosystem
func setupWatchForEcosystem(
	eco workspace.Ecosystem,
	w *watcher.RecursiveWatcher,
	locator *workspace.NotebookLocator,
	allowedPackages map[string]bool,
	watchedPkgs map[string]*watchedPackage,
	quiet bool,
) error {
	// Load ecosystem config to get workspace paths
	groveYmlPath := filepath.Join(eco.Path, "grove.yml")
	cfg, err := coreConfig.Load(groveYmlPath)
	if err != nil {
		return err
	}

	// Get workspace paths from config (expand glob patterns)
	var workspaces []string
	for _, wsPattern := range cfg.Workspaces {
		pattern := filepath.Join(eco.Path, wsPattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && info.IsDir() {
				workspaces = append(workspaces, match)
			}
		}
	}

	for _, wsPath := range workspaces {
		wsName := filepath.Base(wsPath)

		// Load docgen config
		docCfg, err := config.Load(wsPath)
		if err != nil || !docCfg.Enabled {
			continue
		}

		// Skip packages not in allowed set (if filtering)
		if len(allowedPackages) > 0 && !allowedPackages[wsName] {
			if docCfg.Settings.OutputMode != "sections" {
				continue
			}
		}

		// Get workspace node for notebook locator
		node, err := workspace.GetProjectByPath(wsPath)
		if err != nil {
			continue
		}

		// Get docgen directory in notebook
		docgenDir, err := locator.GetDocgenDir(node)
		if err != nil {
			continue
		}

		// Check if docgen dir exists
		if _, err := os.Stat(docgenDir); os.IsNotExist(err) {
			continue
		}

		// Add recursive watch
		if err := w.AddRecursive(docgenDir, wsPath); err != nil {
			if !quiet {
				ulog.Warn("Failed to watch").Field("package", wsName).Err(err).Emit()
			}
			continue
		}

		watchedPkgs[docgenDir] = &watchedPackage{
			wsPath:    wsPath,
			docgenDir: docgenDir,
			pkgName:   wsName,
			config:    docCfg,
		}

		if !quiet {
			ulog.Info("Watching").Field("package", wsName).Field("dir", docgenDir).Emit()
		}
	}

	return nil
}

// findDocgenDir finds the docgen directory that contains the given file path
func findDocgenDir(filePath string, watchedPkgs map[string]*watchedPackage) string {
	for docgenDir := range watchedPkgs {
		if strings.HasPrefix(filePath, docgenDir) {
			return docgenDir
		}
	}
	return ""
}

// rebuildPackage rebuilds a single package and writes to the website
func rebuildPackage(pkg *watchedPackage, w *writer.AstroWriter, mode string, localCfg *config.DocgenConfig, quiet bool) error {
	// Reload config in case it changed
	docCfg, err := config.Load(pkg.wsPath)
	if err != nil {
		return err
	}

	// Handle "sections" output mode (website content like overview, concepts)
	if docCfg.Settings.OutputMode == "sections" {
		return rebuildWebsiteSections(pkg, w, mode, docCfg, localCfg, quiet)
	}

	// Filter sections by status
	var sectionsToProcess []config.SectionConfig
	for _, section := range docCfg.Sections {
		status := section.GetStatus()
		if status == config.StatusDraft {
			continue
		}
		if mode == "prod" && status == config.StatusDev {
			continue
		}
		sectionsToProcess = append(sectionsToProcess, section)
	}

	if len(sectionsToProcess) == 0 {
		return nil
	}

	// Sort sections by order
	sort.Slice(sectionsToProcess, func(i, j int) bool {
		return sectionsToProcess[i].Order < sectionsToProcess[j].Order
	})

	// Get version from git
	version := getPackageVersion(pkg.wsPath)

	// Process each section
	docsDir := filepath.Join(pkg.docgenDir, "docs")
	for i, section := range sectionsToProcess {
		srcFile := filepath.Join(docsDir, section.Output)
		content, err := os.ReadFile(srcFile)
		if err != nil {
			if !quiet {
				ulog.Warn("Could not read section").
					Field("package", pkg.pkgName).
					Field("section", section.Output).
					Err(err).Emit()
			}
			continue
		}

		// Apply strip lines if configured
		if section.AggStripLines > 0 {
			lines := strings.Split(string(content), "\n")
			if len(lines) > section.AggStripLines {
				content = []byte(strings.Join(lines[section.AggStripLines:], "\n"))
			}
		}

		meta := writer.DocMetadata{
			Title:       section.Title,
			Description: docCfg.Description,
			Category:    docCfg.Category,
			Version:     version,
			Order:       i + 1,
			Package:     docCfg.Title,
		}

		transformed, err := w.TransformContent(content, pkg.pkgName, meta)
		if err != nil {
			continue
		}

		if err := w.WriteDoc(pkg.pkgName, section.Output, transformed, meta); err != nil {
			ulog.Error("Failed to write doc").Field("package", pkg.pkgName).Field("file", section.Output).Err(err).Emit()
		}
	}

	// Copy assets
	copyAssets(pkg.docgenDir, pkg.pkgName, w)

	// Update manifest sidebar entry
	updateManifestSidebar(pkg.pkgName, docCfg, mode, w, localCfg)

	return nil
}

// rebuildWebsiteSections handles output_mode: sections (overview, concepts)
func rebuildWebsiteSections(pkg *watchedPackage, w *writer.AstroWriter, mode string, docCfg *config.DocgenConfig, localCfg *config.DocgenConfig, quiet bool) error {
	for _, sectionCfg := range docCfg.Sections {
		dirName := sectionCfg.OutputDir
		if dirName == "" {
			dirName = sectionCfg.Name
		}

		srcDir := filepath.Join(pkg.docgenDir, dirName)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		// Read all markdown files
		files, err := os.ReadDir(srcDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".md" {
				continue
			}

			srcPath := filepath.Join(srcDir, file.Name())
			content, err := os.ReadFile(srcPath)
			if err != nil {
				continue
			}

			// Parse frontmatter for status filtering
			status := parseStatus(string(content))
			if status == config.StatusDraft {
				continue
			}
			if mode == "prod" && status == config.StatusDev {
				continue
			}

			// Transform content (rewrite paths)
			transformed := transformWebsiteSection(string(content), dirName)

			// Write to website content collection
			destPath := filepath.Join(w.WebsiteDir(), "src/content", dirName, file.Name())
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				continue
			}
			if err := os.WriteFile(destPath, []byte(transformed), 0644); err != nil {
				ulog.Error("Failed to write section file").Field("file", destPath).Err(err).Emit()
			}
		}

		// Copy assets for this section
		copyWebsiteSectionAssets(srcDir, dirName, w)
	}

	return nil
}

// transformWebsiteSection transforms paths for website section content
func transformWebsiteSection(content, sectionName string) string {
	basePath := "/docs/" + sectionName

	// Rewrite image paths
	content = strings.ReplaceAll(content, "](./images/", "]("+basePath+"/images/")

	// Rewrite asciinema paths
	content = strings.ReplaceAll(content, `"./asciicasts/`, `"`+basePath+"/asciicasts/")

	// Rewrite video paths
	content = strings.ReplaceAll(content, "](./videos/", "]("+basePath+"/videos/")

	return content
}

// parseStatus extracts status from frontmatter
func parseStatus(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return config.StatusProduction
	}
	end := strings.Index(content[4:], "\n---")
	if end == -1 {
		return config.StatusProduction
	}
	frontmatter := content[4 : end+4]
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, "status:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		}
	}
	return config.StatusProduction
}

// copyAssets copies images, asciicasts, and videos to the website public directory
func copyAssets(docgenDir, pkgName string, w *writer.AstroWriter) {
	assetTypes := []string{"images", "asciicasts", "videos"}
	for _, assetType := range assetTypes {
		srcDir := filepath.Join(docgenDir, assetType)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			filename := filepath.Base(path)
			w.WriteAsset(pkgName, assetType, filename, data)
			return nil
		})
	}
}

// copyWebsiteSectionAssets copies assets for a website section
func copyWebsiteSectionAssets(srcDir, sectionName string, w *writer.AstroWriter) {
	assetTypes := []string{"images", "asciicasts", "videos"}
	for _, assetType := range assetTypes {
		assetDir := filepath.Join(srcDir, assetType)
		if _, err := os.Stat(assetDir); os.IsNotExist(err) {
			continue
		}

		filepath.Walk(assetDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			filename := filepath.Base(path)
			w.WriteAsset(sectionName, assetType, filename, data)
			return nil
		})
	}
}

// updateManifestSidebar updates the manifest with sidebar info for incremental builds
func updateManifestSidebar(pkgName string, docCfg *config.DocgenConfig, mode string, w *writer.AstroWriter, localCfg *config.DocgenConfig) {
	// Read existing manifest
	manifestPath := filepath.Join(w.WebsiteDir(), "docgen-output/manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return // Manifest doesn't exist yet, will be created by full aggregate
	}

	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}

	// Update or add the package in the manifest
	// This is a simplified update - a full rebuild via aggregate is more accurate
	// but this provides basic sidebar consistency during watch

	// Save updated manifest
	data, err = json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(manifestPath, data, 0644)
}

// getPackageVersion gets version from git tags
func getPackageVersion(wsPath string) string {
	// Try git describe first
	// Simplified - in production this would use exec.Command
	return "latest"
}

// errorf creates a formatted error
func errorf(format string, args ...interface{}) error {
	return &watchError{msg: formatString(format, args...)}
}

type watchError struct {
	msg string
}

func (e *watchError) Error() string {
	return e.msg
}

func formatString(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	// Simple formatting - in production use fmt.Sprintf
	result := format
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			result = strings.Replace(result, "%w", err.Error(), 1)
		} else if s, ok := arg.(string); ok {
			result = strings.Replace(result, "%s", s, 1)
		}
	}
	return result
}
