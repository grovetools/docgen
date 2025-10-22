package generator

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/mattsolo1/grove-docgen/pkg/parser"
	"github.com/sirupsen/logrus"
)

// Generator handles the documentation generation for a single package.
type Generator struct {
	logger *logrus.Logger
}

// GenerateOptions configures what sections to generate
type GenerateOptions struct {
	Sections []string // List of section names to generate (empty means all)
}

func New(logger *logrus.Logger) *Generator {
	return &Generator{logger: logger}
}

// Generate orchestrates an isolated documentation generation process for all sections.
func (g *Generator) Generate(packageDir string) error {
	return g.GenerateWithOptions(packageDir, GenerateOptions{})
}

// GenerateWithOptions orchestrates documentation generation with specific options.
func (g *Generator) GenerateWithOptions(packageDir string, opts GenerateOptions) error {
	if len(opts.Sections) > 0 {
		g.logger.Infof("Starting generation for package at: %s (sections: %v)", packageDir, opts.Sections)
	} else {
		g.logger.Infof("Starting generation for package at: %s", packageDir)
	}

	// Run the generation logic directly in the package directory
	if err := g.generateInPlace(packageDir, opts); err != nil {
		return fmt.Errorf("generation process failed: %w", err)
	}

	// Generate JSON from markdown if configured
	cfg, err := config.Load(packageDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Settings.StructuredOutputFile != "" {
		g.logger.Info("Generating structured JSON from markdown...")
		p := parser.New(g.logger)
		if err := p.GenerateJSON(packageDir, cfg); err != nil {
			g.logger.WithError(err).Error("Failed to generate JSON from markdown")
			// Don't fail the whole process if JSON generation fails
		}
	}

	return nil
}

// generateInPlace runs the core doc generation logic within a given directory.
func (g *Generator) generateInPlace(packageDir string, opts GenerateOptions) error {
	g.logger.Infof("Generating documentation in: %s", packageDir)

	// 1. Load config from the package directory
	cfg, err := config.Load(packageDir)
	if err != nil {
		return fmt.Errorf("failed to load docgen config: %w", err)
	}

	// 2. Setup rules file if specified
	if cfg.Settings.RulesFile != "" {
		if err := g.setupRulesFile(packageDir, cfg.Settings.RulesFile); err != nil {
			return fmt.Errorf("failed to setup rules file: %w", err)
		}
	}

	// 3. Build context using `cx`
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.buildContext(packageDir); err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	// 3. Load system prompt if configured
	systemPrompt := ""
	if cfg.Settings.SystemPrompt != "" {
		if cfg.Settings.SystemPrompt == "default" {
			systemPrompt = DefaultSystemPrompt
			g.logger.Debug("Using default system prompt")
		} else {
			// Load custom system prompt file
			systemPromptPath := filepath.Join(packageDir, "docs", cfg.Settings.SystemPrompt)
			if content, err := os.ReadFile(systemPromptPath); err == nil {
				systemPrompt = string(content)
				g.logger.Debugf("Loaded system prompt from %s", cfg.Settings.SystemPrompt)
			} else {
				g.logger.Warnf("Failed to load system prompt from %s, proceeding without it", cfg.Settings.SystemPrompt)
			}
		}
	}

	// 4. Filter sections if specified
	sectionsToGenerate := cfg.Sections
	if len(opts.Sections) > 0 {
		// Create a map for quick lookup
		requestedSections := make(map[string]bool)
		for _, name := range opts.Sections {
			requestedSections[name] = true
		}
		
		// Filter sections and validate
		var filteredSections []config.SectionConfig
		var invalidSections []string
		
		for _, section := range cfg.Sections {
			// Check if this section was requested
			if requestedSections[section.Name] {
				filteredSections = append(filteredSections, section)
				delete(requestedSections, section.Name) // Remove from map to track found sections
			}
		}
		
		// Check for any requested sections that weren't found
		for name := range requestedSections {
			invalidSections = append(invalidSections, name)
		}
		
		if len(invalidSections) > 0 {
			return fmt.Errorf("sections not found in config: %v", invalidSections)
		}
		
		sectionsToGenerate = filteredSections
		g.logger.Infof("Generating %d of %d sections: %v", len(sectionsToGenerate), len(cfg.Sections), opts.Sections)
	}
	
	// 5. Generate each section
	for _, section := range sectionsToGenerate {
		g.logger.Infof("Generating section: %s", section.Name)

		promptPath := filepath.Join(packageDir, "docs", section.Prompt)
		promptContent, err := os.ReadFile(promptPath)
		if err != nil {
			return fmt.Errorf("failed to read prompt file %s: %w", promptPath, err)
		}

		// Build the final prompt with system prompt prepended if available
		finalPrompt := string(promptContent)
		if systemPrompt != "" {
			finalPrompt = systemPrompt + "\n" + finalPrompt
		}

		// Handle reference mode
		if cfg.Settings.RegenerationMode == "reference" {
			// Determine the output path based on OutputDir setting
			outputDir := cfg.Settings.OutputDir
			if outputDir == "" {
				outputDir = "docs"
			}
			existingOutputPath := filepath.Join(packageDir, outputDir, section.Output)
			if existingDocs, err := os.ReadFile(existingOutputPath); err == nil {
				g.logger.Debugf("Injecting reference content from %s", existingOutputPath)
				finalPrompt = "For your reference, here is the previous version of the documentation:\n\n<reference_docs>\n" +
					string(existingDocs) + "\n</reference_docs>\n\n---\n\n" + finalPrompt
			}
		}

		// Determine model to use (section override or global)
		model := cfg.Settings.Model
		if section.Model != "" {
			model = section.Model
			g.logger.Debugf("Using section-specific model: %s", model)
		}

		// Merge generation configs (global + section overrides)
		genConfig := config.MergeGenerationConfig(cfg.Settings.GenerationConfig, section.GenerationConfig)

		output, err := g.callLLM(finalPrompt, model, genConfig, packageDir)
		if err != nil {
			g.logger.WithError(err).Errorf("LLM call failed for section '%s'", section.Name)
			continue // Continue to the next section even if one fails
		}

		// 6. Write output to the configured output directory
		outputDir := cfg.Settings.OutputDir
		if outputDir == "" {
			outputDir = "docs"
		}
		outputPath := filepath.Join(packageDir, outputDir, section.Output)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write section output: %w", err)
		}
		g.logger.Infof("Successfully wrote section '%s' to %s", section.Name, outputPath)
	}

	return nil
}

func (g *Generator) setupRulesFile(packageDir, rulesFile string) error {
	// Read the specified rules file
	rulesPath := filepath.Join(packageDir, "docs", rulesFile)
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		return fmt.Errorf("failed to read rules file %s: %w", rulesPath, err)
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(packageDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("failed to create .grove directory: %w", err)
	}

	// Copy the rules file content to .grove/rules
	// Since we're now operating locally, no path adjustments are needed
	groveRulesPath := filepath.Join(groveDir, "rules")
	if err := os.WriteFile(groveRulesPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write .grove/rules: %w", err)
	}

	g.logger.Debugf("Setup rules file from %s to .grove/rules", rulesFile)
	return nil
}

func (g *Generator) buildContext(packageDir string) error {
	// Use 'grove cx generate' for workspace-awareness
	cmd := exec.Command("grove", "cx", "generate")
	cmd.Dir = packageDir
	// Discard output to avoid contaminating the LLM response
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func (g *Generator) callLLM(promptContent, model string, genConfig config.GenerationConfig, workDir string) (string, error) {
	// Use provided model or default to gemini-1.5-flash-latest
	if model == "" {
		model = "gemini-1.5-flash-latest"
	}

	// Create a temporary file for the prompt
	promptFile, err := os.CreateTemp("", "docgen-prompt-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp prompt file: %w", err)
	}
	defer os.Remove(promptFile.Name())

	if _, err := promptFile.WriteString(promptContent); err != nil {
		return "", fmt.Errorf("failed to write to temp prompt file: %w", err)
	}
	if err := promptFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp prompt file: %w", err)
	}

	// Use the grove llm facade to make the request
	args := []string{
		"llm",
		"request",
		"--model", model,
		"--file", promptFile.Name(),
		"--yes",
	}

	// Add generation parameters if specified
	if genConfig.Temperature != nil {
		args = append(args, "--temperature", fmt.Sprintf("%.2f", *genConfig.Temperature))
	}
	if genConfig.TopP != nil {
		args = append(args, "--top-p", fmt.Sprintf("%.2f", *genConfig.TopP))
	}
	if genConfig.TopK != nil {
		args = append(args, "--top-k", fmt.Sprintf("%d", *genConfig.TopK))
	}
	if genConfig.MaxOutputTokens != nil {
		args = append(args, "--max-output-tokens", fmt.Sprintf("%d", *genConfig.MaxOutputTokens))
	}

	cmd := exec.Command("grove", args...)
	cmd.Dir = workDir // Run in the isolated clone directory
	// Let stderr go to console for debug output, capture only stdout
	cmd.Stderr = os.Stderr
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("grove llm request failed: %w", err)
	}
	
	// Clean up the output
	response := string(output)
	response = strings.TrimSpace(response)
	
	// Remove markdown code fences if present
	if strings.HasPrefix(response, "```markdown") || strings.HasPrefix(response, "```md") {
		lines := strings.Split(response, "\n")
		if len(lines) > 2 && strings.HasSuffix(response, "```") {
			response = strings.Join(lines[1:len(lines)-1], "\n")
		}
	} else if strings.HasPrefix(response, "```") && strings.HasSuffix(response, "```") {
		lines := strings.Split(response, "\n")
		if len(lines) > 2 {
			response = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	
	// Clean up any remaining issues
	// The response should be clean markdown at this point
	
	return response, nil
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