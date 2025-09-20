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

// GenerateWithOptions orchestrates an isolated documentation generation with specific options.
func (g *Generator) GenerateWithOptions(packageDir string, opts GenerateOptions) error {
	if len(opts.Sections) > 0 {
		g.logger.Infof("Starting isolated generation for package at: %s (sections: %v)", packageDir, opts.Sections)
	} else {
		g.logger.Infof("Starting isolated generation for package at: %s", packageDir)
	}

	// 1. Create temporary directory for the clone
	tempDir, err := os.MkdirTemp("", "docgen-isolated-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		g.logger.Debugf("Cleaning up temporary directory: %s", tempDir)
		os.RemoveAll(tempDir)
	}()
	g.logger.Debugf("Created temporary directory: %s", tempDir)

	// 2. Perform a local clone
	g.logger.Debug("Cloning repository locally for isolation...")
	cloneCmd := exec.Command("git", "clone", "--local", ".", tempDir)
	cloneCmd.Dir = packageDir
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repository into temp dir: %w\nOutput: %s", err, string(output))
	}

	// 3. Run the generation logic inside the isolated environment
	if err := g.generateInPlace(tempDir, packageDir, opts); err != nil {
		return fmt.Errorf("generation process failed in isolation: %w", err)
	}

	// 4. Copy generated markdown files back (only the output files)
	cfg, err := config.Load(packageDir)
	if err != nil {
		return fmt.Errorf("failed to load config for copying files: %w", err)
	}
	
	// Determine which sections to copy back
	sectionsToCopy := cfg.Sections
	if len(opts.Sections) > 0 {
		// Only copy back the sections that were requested
		requestedMap := make(map[string]bool)
		for _, name := range opts.Sections {
			requestedMap[name] = true
		}
		
		var filtered []config.SectionConfig
		for _, section := range cfg.Sections {
			if requestedMap[section.Name] {
				filtered = append(filtered, section)
			}
		}
		sectionsToCopy = filtered
	}
	
	for _, section := range sectionsToCopy {
		srcPath := filepath.Join(tempDir, "docs", section.Output)
		destPath := filepath.Join(packageDir, "docs", section.Output)
		
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			g.logger.Warnf("Generated file %s does not exist", srcPath)
			continue
		}
		
		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
		
		// Copy the file
		srcData, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read generated file %s: %w", srcPath, err)
		}
		
		if err := os.WriteFile(destPath, srcData, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}
		
		g.logger.Infof("Copied %s to %s", section.Output, destPath)
	}

	// 5. Generate JSON from markdown if configured
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
func (g *Generator) generateInPlace(cloneDir, originalDir string, opts GenerateOptions) error {
	g.logger.Infof("Generating documentation within isolated directory: %s", cloneDir)

	// 1. Load config from the cloned directory
	cfg, err := config.Load(cloneDir)
	if err != nil {
		return fmt.Errorf("failed to load docgen config from temp dir: %w", err)
	}

	// 2. Setup rules file if specified
	if cfg.Settings.RulesFile != "" {
		if err := g.setupRulesFile(cloneDir, originalDir, cfg.Settings.RulesFile); err != nil {
			return fmt.Errorf("failed to setup rules file: %w", err)
		}
	}

	// 3. Build context using `cx`
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.buildContext(cloneDir); err != nil {
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
			systemPromptPath := filepath.Join(cloneDir, "docs", cfg.Settings.SystemPrompt)
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

		promptPath := filepath.Join(cloneDir, "docs", section.Prompt)
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
			originalOutputPath := filepath.Join(originalDir, "docs", section.Output)
			if existingDocs, err := os.ReadFile(originalOutputPath); err == nil {
				g.logger.Debugf("Injecting reference content from %s", originalOutputPath)
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

		output, err := g.callLLM(finalPrompt, model, genConfig, cloneDir)
		if err != nil {
			g.logger.WithError(err).Errorf("LLM call failed for section '%s'", section.Name)
			continue // Continue to the next section even if one fails
		}

		// 6. Write output
		outputPath := filepath.Join(cloneDir, "docs", section.Output)
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

func (g *Generator) setupRulesFile(cloneDir, originalDir, rulesFile string) error {
	// Read the specified rules file
	rulesPath := filepath.Join(cloneDir, "docs", rulesFile)
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		return fmt.Errorf("failed to read rules file %s: %w", rulesPath, err)
	}

	// Adjust relative paths for isolation
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check if line starts with ../
		if strings.HasPrefix(trimmed, "../") {
			// Get the grove-ecosystem root, handling worktrees
			var groveEcosystemRoot string
			
			// Check if we're in a worktree by looking for .grove-worktrees in the path
			if strings.Contains(originalDir, ".grove-worktrees") {
				// Extract the path up to the package (before .grove-worktrees)
				parts := strings.Split(originalDir, ".grove-worktrees")
				if len(parts) > 0 {
					// parts[0] should be /path/to/grove-ecosystem/package-name/
					// We want the parent of the package
					packagePath := strings.TrimSuffix(parts[0], "/")
					groveEcosystemRoot = filepath.Dir(packagePath)
				}
			} else {
				// Regular checkout - just get the parent directory
				groveEcosystemRoot = filepath.Dir(originalDir)
			}
			
			// Remove the leading ../ and resolve the absolute path
			relativePart := strings.TrimPrefix(trimmed, "../")
			absolutePath := filepath.Join(groveEcosystemRoot, relativePart)
			
			// Validate path exists (check up to the last non-glob part)
			pathParts := strings.Split(relativePart, "/")
			var checkPath string
			for j, part := range pathParts {
				if strings.Contains(part, "*") || strings.Contains(part, "?") || strings.Contains(part, "[") {
					// Found a glob pattern, use path up to previous part
					if j > 0 {
						checkPath = filepath.Join(groveEcosystemRoot, strings.Join(pathParts[:j], "/"))
					}
					break
				}
			}
			// If no glob found, check the full path
			if checkPath == "" && len(pathParts) > 0 {
				checkPath = filepath.Join(groveEcosystemRoot, strings.Split(relativePart, "*")[0])
			}
			
			if checkPath != "" {
				if _, err := os.Stat(checkPath); os.IsNotExist(err) {
					g.logger.Debugf("Path doesn't exist, commenting out: %s", trimmed)
					lines[i] = "# " + line // Comment out the line
					continue
				}
			}
			
			lines[i] = absolutePath
			g.logger.Debugf("Converted relative path %s to %s", trimmed, absolutePath)
		}
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(cloneDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("failed to create .grove directory: %w", err)
	}

	// Write adjusted content to .grove/rules
	adjustedContent := strings.Join(lines, "\n")
	groveRulesPath := filepath.Join(groveDir, "rules")
	if err := os.WriteFile(groveRulesPath, []byte(adjustedContent), 0644); err != nil {
		return fmt.Errorf("failed to write .grove/rules: %w", err)
	}

	g.logger.Debugf("Setup rules file from %s to .grove/rules", rulesFile)
	return nil
}

func (g *Generator) buildContext(packageDir string) error {
	// cx generate uses the .grove/rules file if it exists, relative to the CWD.
	cmd := exec.Command("cx", "generate")
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

	// Use gemapi to make the request (following grove-tend's original approach)
	args := []string{
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
	
	cmd := exec.Command("gemapi", args...)
	cmd.Dir = workDir // Run in the isolated clone directory
	// Let stderr go to console for debug output, capture only stdout
	cmd.Stderr = os.Stderr
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gemapi request failed: %w", err)
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