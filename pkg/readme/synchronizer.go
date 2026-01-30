package readme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/grovetools/docgen/pkg/config"
	"github.com/grovetools/docgen/pkg/logo"
	"github.com/sirupsen/logrus"
)

// Synchronizer handles the process of generating a README.md from a template and documentation source.
type Synchronizer struct {
	logger *logrus.Logger
}

// New creates a new Synchronizer instance.
func New(logger *logrus.Logger) *Synchronizer {
	return &Synchronizer{logger: logger}
}

// Sync performs the README synchronization for a given package directory.
func (s *Synchronizer) Sync(packageDir string) error {
	cfg, configPath, err := config.LoadWithNotebook(packageDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Debugf("Skipping README sync: no docgen.config.yml found in %s", packageDir)
			return nil
		}
		return fmt.Errorf("failed to load docgen.config.yml: %w", err)
	}

	if cfg.Readme == nil {
		s.logger.Info("Skipping README sync: 'readme' section not configured in docgen.config.yml")
		return nil
	}
	s.logger.Debugf("Starting README sync for %s...", cfg.Title)

	// Validate configuration
	if cfg.Readme.Template == "" || cfg.Readme.Output == "" || cfg.Readme.SourceSection == "" {
		return fmt.Errorf("invalid readme configuration: template, output, and source_section must all be set")
	}

	// Find the source section config
	var sourceSectionConfig *config.SectionConfig
	for i, section := range cfg.Sections {
		if section.Name == cfg.Readme.SourceSection {
			sourceSectionConfig = &cfg.Sections[i]
			break
		}
	}
	if sourceSectionConfig == nil {
		return fmt.Errorf("source_section '%s' not found in docgen.config.yml sections", cfg.Readme.SourceSection)
	}

	// Check if the source documentation file exists
	// If config is in notebook, look for docs in notebook's docgen/docs/
	// Otherwise look in package directory
	var sourceDocPath string
	if strings.Contains(configPath, ".grove/notebooks") || strings.Contains(configPath, "/.notebook/") {
		// Notebook mode: docs are in docgen/docs/
		configDir := filepath.Dir(configPath)
		sourceDocPath = filepath.Join(configDir, "docs", sourceSectionConfig.Output)
		s.logger.Debugf("Looking for source doc in notebook: %s", sourceDocPath)
	} else {
		// Repo mode: docs are in configured output_dir
		outputDir := cfg.Settings.OutputDir
		if outputDir == "" {
			outputDir = "docs"
		}
		sourceDocPath = filepath.Join(packageDir, outputDir, sourceSectionConfig.Output)
		s.logger.Debugf("Looking for source doc in repo: %s", sourceDocPath)
	}

	if _, err := os.Stat(sourceDocPath); os.IsNotExist(err) {
		return fmt.Errorf("source documentation file not found: %s. Run 'docgen generate --section %s' first", sourceDocPath, cfg.Readme.SourceSection)
	}

	// Read source documentation content
	sourceContent, err := os.ReadFile(sourceDocPath)
	if err != nil {
		return fmt.Errorf("failed to read source documentation file %s: %w", sourceDocPath, err)
	}

	// Strip specified number of lines from the top if configured
	if cfg.Readme.StripLines > 0 {
		lines := strings.Split(string(sourceContent), "\n")
		if len(lines) > cfg.Readme.StripLines {
			// Join the remaining lines after stripping
			sourceContent = []byte(strings.Join(lines[cfg.Readme.StripLines:], "\n"))
		} else {
			// If file has fewer lines than strip_lines, result is empty
			sourceContent = []byte("")
			s.logger.Warnf("Source file has fewer lines (%d) than strip_lines setting (%d)", len(lines), cfg.Readme.StripLines)
		}
	}

	// Read template content
	// If template path starts with docgen/, resolve from config location (notebook)
	// Otherwise resolve from package directory (legacy)
	var templatePath string
	if strings.HasPrefix(cfg.Readme.Template, "docgen/") {
		// Template is in notebook's docgen directory
		configDir := filepath.Dir(configPath)
		// Remove "docgen/" prefix since we're already in the config dir (which is docgen/)
		templateRelPath := strings.TrimPrefix(cfg.Readme.Template, "docgen/")
		templatePath = filepath.Join(configDir, templateRelPath)
		s.logger.Debugf("Resolving template from notebook: %s", templatePath)
	} else {
		// Legacy: template in package directory
		templatePath = filepath.Join(packageDir, cfg.Readme.Template)
		s.logger.Debugf("Resolving template from package: %s", templatePath)
	}

	templateContentBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read README template file %s: %w", templatePath, err)
	}
	templateContent := string(templateContentBytes)

	// --- Perform Replacements ---

	// 1. Replace metadata variables
	replacer := strings.NewReplacer(
		"{{ .Title }}", cfg.Title,
		"{{ .Description }}", cfg.Description,
		"{{ .PackageName }}", filepath.Base(packageDir),
	)
	composedContent := replacer.Replace(templateContent)

	// 2. Replace source section content
	startMarker := fmt.Sprintf("<!-- DOCGEN:%s:START -->", strings.ToUpper(cfg.Readme.SourceSection))
	endMarker := fmt.Sprintf("<!-- DOCGEN:%s:END -->", strings.ToUpper(cfg.Readme.SourceSection))

	startIdx := strings.Index(composedContent, startMarker)
	endIdx := strings.Index(composedContent, endMarker)

	if startIdx == -1 || endIdx == -1 {
		s.logger.Warnf("Could not find markers %s and %s in template. Skipping content injection.", startMarker, endMarker)
	} else {
		// Rewrite media paths for the README context
		rewrittenSource := rewriteMediaPathsForReadme(string(sourceContent), cfg.Readme.BaseURL)
		
		prefix := composedContent[:startIdx+len(startMarker)]
		suffix := composedContent[endIdx:]
		composedContent = prefix + "\n\n" + strings.TrimSpace(rewrittenSource) + "\n\n" + suffix
	}

	// 3. Generate and inject TOC if enabled
	if cfg.Readme.GenerateTOC {
		err := s.injectTOC(&composedContent, cfg, packageDir)
		if err != nil {
			s.logger.Warnf("Failed to generate TOC: %v", err)
		}
	}

	// 4. Rewrite all media paths in the final content (including template paths)
	if cfg.Readme.BaseURL != "" {
		composedContent = rewriteMediaPathsForReadme(composedContent, cfg.Readme.BaseURL)
	}

	// Write the final README.md
	outputPath := filepath.Join(packageDir, cfg.Readme.Output)
	if err := os.WriteFile(outputPath, []byte(composedContent), 0644); err != nil {
		return fmt.Errorf("failed to write output README file %s: %w", outputPath, err)
	}

	s.logger.Debugf("Successfully synchronized %s", outputPath)

	// 5. Generate logo with text if configured
	if cfg.Readme.Logo != nil {
		if err := s.generateLogo(cfg.Readme.Logo, packageDir); err != nil {
			return fmt.Errorf("failed to generate logo: %w", err)
		}
	}

	return nil
}

// generateLogo creates a combined logo+text SVG using the logo package.
func (s *Synchronizer) generateLogo(logoCfg *config.LogoConfig, packageDir string) error {
	// Helper to expand ~ in paths
	expandPath := func(p string) string {
		if strings.HasPrefix(p, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				return filepath.Join(home, p[2:])
			}
		}
		return p
	}

	// Resolve input path - could be absolute or relative to package
	inputPath := expandPath(logoCfg.Input)
	if !filepath.IsAbs(inputPath) {
		inputPath = filepath.Join(packageDir, inputPath)
	}

	// Resolve output path
	outputPath := expandPath(logoCfg.Output)
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(packageDir, outputPath)
	}

	// Expand ~ in font path
	fontPath := expandPath(logoCfg.Font)

	// Apply defaults
	spacing := logoCfg.Spacing
	if spacing == 0 {
		spacing = 35
	}
	textScale := logoCfg.TextScale
	if textScale == 0 {
		textScale = 1.1
	}
	color := logoCfg.Color
	if color == "" {
		color = "#589ac7"
	}
	width := logoCfg.Width
	if width == 0 {
		width = 200
	}

	cfg := logo.Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Text:       logoCfg.Text,
		TextColor:  color,
		FontPath:   fontPath,
		Spacing:    spacing,
		TextScale:  textScale,
		Width:      width,
	}

	gen := logo.New(s.logger)
	if err := gen.Generate(cfg); err != nil {
		return err
	}

	s.logger.Debugf("Generated logo with text: %s", outputPath)
	return nil
}

// rewriteMediaPathsForReadme rewrites media paths in markdown content for README context.
// If baseURL is provided, root-relative paths (starting with /) are converted to absolute URLs.
// Relative paths (like ./images/) are converted to docs/ paths for the repository context.
func rewriteMediaPathsForReadme(content, baseURL string) string {
	// First handle markdown image/link syntax: ![alt text](path) and [![alt](img)](link)
	markdownRe := regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]*)\)`)
	content = markdownRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := markdownRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		altText := parts[1]
		path := parts[2]

		newPath := rewritePath(path, baseURL)
		if newPath == path {
			return match
		}
		return fmt.Sprintf("![%s](%s)", altText, newPath)
	})

	// Handle markdown links that wrap images: [![...](img)](link)
	// This catches video thumbnails linked to video files
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\(([^\)]+)\)`)
	content = linkRe.ReplaceAllStringFunc(content, func(match string) string {
		// Skip if it's an image (already handled above)
		if strings.HasPrefix(match, "!") {
			return match
		}
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		linkText := parts[1]
		path := parts[2]

		newPath := rewritePath(path, baseURL)
		if newPath == path {
			return match
		}
		return fmt.Sprintf("[%s](%s)", linkText, newPath)
	})

	// Then handle HTML img tags: <img src="path" ...>
	htmlRe := regexp.MustCompile(`<img\s+([^>]*\s)?src="([^"]+)"([^>]*)>`)
	content = htmlRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := htmlRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		beforeSrc := parts[1]
		if beforeSrc == "" {
			beforeSrc = ""
		}
		path := parts[2]
		afterSrc := parts[3]

		newPath := rewritePath(path, baseURL)
		if newPath == path {
			return match
		}
		return fmt.Sprintf(`<img %ssrc="%s"%s>`, beforeSrc, newPath, afterSrc)
	})

	return content
}

// rewritePath rewrites a single path according to README context rules.
// - External URLs (http/https) are unchanged
// - Root-relative paths (/) are prefixed with baseURL if provided, otherwise unchanged
// - Relative paths (./images/) are converted to docs/images/
// - Other relative paths are prefixed with docs/
func rewritePath(path, baseURL string) string {
	// External URLs are unchanged
	if strings.HasPrefix(path, "http") {
		return path
	}

	// Root-relative paths: convert to absolute URL if baseURL is set
	if strings.HasPrefix(path, "/") {
		if baseURL != "" {
			return strings.TrimRight(baseURL, "/") + path
		}
		// No baseURL, leave root-relative paths unchanged
		return path
	}

	// Already has docs/ prefix
	if strings.HasPrefix(path, "docs/") {
		return path
	}

	// Handle ./images/ paths - convert to docs/images/
	if strings.HasPrefix(path, "./images/") {
		return "docs/images/" + strings.TrimPrefix(path, "./images/")
	}

	// For other relative paths, prepend "docs/"
	return "docs/" + path
}

// injectTOC generates and injects a table of contents into the README content.
func (s *Synchronizer) injectTOC(content *string, cfg *config.DocgenConfig, packageDir string) error {
	// Look for TOC markers
	startMarker := "<!-- DOCGEN:TOC:START -->"
	endMarker := "<!-- DOCGEN:TOC:END -->"

	startIdx := strings.Index(*content, startMarker)
	endIdx := strings.Index(*content, endMarker)

	if startIdx == -1 || endIdx == -1 {
		s.logger.Debugf("No TOC markers found in template. Skipping TOC generation.")
		return nil
	}

	// Generate TOC content
	tocContent, err := s.generateTOC(cfg, packageDir)
	if err != nil {
		return fmt.Errorf("failed to generate TOC: %w", err)
	}

	// Replace content between markers
	prefix := (*content)[:startIdx+len(startMarker)]
	suffix := (*content)[endIdx:]
	*content = prefix + "\n\n" + tocContent + "\n\n" + suffix

	return nil
}

// generateTOC creates a markdown table of contents from the documentation sections.
func (s *Synchronizer) generateTOC(cfg *config.DocgenConfig, packageDir string) (string, error) {
	outputDir := cfg.Settings.OutputDir
	if outputDir == "" {
		outputDir = "docs"
	}

	var tocLines []string
	tocLines = append(tocLines, "See the [documentation]("+outputDir+"/) for detailed usage instructions:")

	// Sort sections by order
	sections := cfg.Sections
	for i := 0; i < len(sections); i++ {
		for j := i + 1; j < len(sections); j++ {
			if sections[j].Order < sections[i].Order {
				sections[i], sections[j] = sections[j], sections[i]
			}
		}
	}

	// Generate TOC entries for each section
	for _, section := range sections {
		// Check if the documentation file exists
		docPath := filepath.Join(packageDir, outputDir, section.Output)
		if _, err := os.Stat(docPath); os.IsNotExist(err) {
			s.logger.Debugf("Skipping TOC entry for %s: file not found at %s", section.Name, docPath)
			continue
		}

		// Create clean TOC entry without description to avoid broken images
		tocEntry := fmt.Sprintf("- [%s](%s/%s)", section.Title, outputDir, section.Output)
		tocLines = append(tocLines, tocEntry)
	}

	return strings.Join(tocLines, "\n"), nil
}

// extractDescription attempts to extract a brief description from a documentation file.
func (s *Synchronizer) extractDescription(filePath, title string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	
	// Skip the title line and empty lines, look for the first substantial line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip title lines (starting with #)
		if strings.HasPrefix(line, "#") {
			continue
		}
		
		// Skip empty lines
		if line == "" {
			continue
		}
		
		// Take the first substantial line as description
		if len(line) > 20 { // Only use if it's substantial
			// Truncate if too long
			if len(line) > 80 {
				line = line[:77] + "..."
			}
			return line
		}
	}
	
	return ""
}