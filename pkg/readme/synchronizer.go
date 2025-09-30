package readme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-docgen/pkg/config"
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
	cfg, err := config.Load(packageDir)
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
	s.logger.Infof("Starting README sync for %s...", cfg.Title)

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
	outputDir := cfg.Settings.OutputDir
	if outputDir == "" {
		outputDir = "docs"
	}
	sourceDocPath := filepath.Join(packageDir, outputDir, sourceSectionConfig.Output)
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
	templatePath := filepath.Join(packageDir, cfg.Readme.Template)
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
		prefix := composedContent[:startIdx+len(startMarker)]
		suffix := composedContent[endIdx:]
		composedContent = prefix + "\n\n" + strings.TrimSpace(string(sourceContent)) + "\n\n" + suffix
	}

	// Write the final README.md
	outputPath := filepath.Join(packageDir, cfg.Readme.Output)
	if err := os.WriteFile(outputPath, []byte(composedContent), 0644); err != nil {
		return fmt.Errorf("failed to write output README file %s: %w", outputPath, err)
	}

	s.logger.Infof("✓ Successfully synchronized %s", outputPath)
	return nil
}