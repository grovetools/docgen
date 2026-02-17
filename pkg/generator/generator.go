package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/logging"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/core/util/delegation"
	"github.com/grovetools/docgen/pkg/capture"
	"github.com/grovetools/docgen/pkg/config"
	"github.com/grovetools/docgen/pkg/parser"
	"github.com/grovetools/docgen/pkg/schema"
	"github.com/sirupsen/logrus"
)

var ulog = logging.NewUnifiedLogger("grove-docgen")

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

// resolvePromptContent finds and reads a prompt file, trying notebook location first.
// It follows this resolution order:
// 1. Tries to resolve the workspace and get the notebook prompts directory
// 2. Looks for the prompt in the notebook directory (using basename only)
// 3. Falls back to the legacy path in docs/prompts/
// Returns the prompt content or an error if not found in either location.
func (g *Generator) resolvePromptContent(packageDir, promptFile string) ([]byte, error) {
	// Extract basename only - ignore any directory prefix for backward compatibility
	promptBaseName := filepath.Base(promptFile)

	// 1. Try to get workspace node for the package directory
	node, err := workspace.GetProjectByPath(packageDir)
	if err != nil {
		// Fallback: Can't resolve workspace, use legacy path
		g.logger.Warnf("Could not resolve workspace for %s. Falling back to legacy prompt path.", packageDir)
		legacyPath := filepath.Join(packageDir, "docs", promptFile)
		return os.ReadFile(legacyPath)
	}

	// 2. Try notebook path first
	cfg, err := coreConfig.LoadDefault()
	if err == nil {
		locator := workspace.NewNotebookLocator(cfg)
		notebookPromptsDir, err := locator.GetDocgenPromptsDir(node)

		if err == nil {
			notebookPath := filepath.Join(notebookPromptsDir, promptBaseName)
			if data, err := os.ReadFile(notebookPath); err == nil {
				g.logger.Debugf("Loaded prompt '%s' from notebook: %s", promptBaseName, notebookPath)
				return data, nil
			}
		}
	}

	// 3. Fallback to legacy path
	legacyPath := filepath.Join(packageDir, "docs", promptFile)
	g.logger.Debugf("Prompt not found in notebook, trying legacy path: %s", legacyPath)
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		// Enhanced error message showing both paths attempted
		notebookPromptsDir := "unable to resolve"
		if cfg, cfgErr := coreConfig.LoadDefault(); cfgErr == nil {
			locator := workspace.NewNotebookLocator(cfg)
			if dir, dirErr := locator.GetDocgenPromptsDir(node); dirErr == nil {
				notebookPromptsDir = dir
			}
		}
		return nil, fmt.Errorf(
			"prompt '%s' not found in notebook (%s) or legacy location (%s)",
			promptBaseName, notebookPromptsDir, legacyPath,
		)
	}

	return data, nil
}

// generateInPlace runs the core doc generation logic within a given directory.
func (g *Generator) generateInPlace(packageDir string, opts GenerateOptions) error {
	g.logger.Infof("Generating documentation in: %s", packageDir)

	// 1. Load config from the package directory (tries notebook first, then repo)
	cfg, configPath, err := config.LoadWithNotebook(packageDir)
	if err != nil {
		return fmt.Errorf("failed to load docgen config: %w", err)
	}

	// Handle "sections" output mode: delegate to subdirectory-based generation
	if cfg.Settings.OutputMode == "sections" {
		return g.generateSectionsMode(packageDir, configPath, cfg, opts)
	}

	// 2. Determine output base directory based on config location
	var outputBaseDir string

	// Check if config was loaded from notebook by checking if config path is outside the repo
	// (notebook configs won't be under packageDir)
	isNotebookMode := !strings.HasPrefix(configPath, packageDir)
	if isNotebookMode {
		// Output to notebook's docgen/docs/ directory
		docgenDir := filepath.Dir(configPath) // configPath is docgenDir/docgen.config.yml
		outputBaseDir = filepath.Join(docgenDir, "docs")
		g.logger.Infof("Using notebook mode: config from %s, outputting to %s", configPath, outputBaseDir)
		ulog.Info("Notebook mode").
			Field("config", configPath).
			Field("output", outputBaseDir).
			Emit()
	} else {
		// Output to repo's configured output_dir (default: docs/)
		if cfg.Settings.OutputDir != "" {
			outputBaseDir = filepath.Join(packageDir, cfg.Settings.OutputDir)
		} else {
			outputBaseDir = filepath.Join(packageDir, "docs")
		}
		g.logger.Infof("Using repo mode: config from %s, outputting to %s", configPath, outputBaseDir)
		ulog.Info("Repo mode").
			Field("config", configPath).
			Field("output", outputBaseDir).
			Emit()
	}

	// 2. Setup rules file if specified
	if cfg.Settings.RulesFile != "" {
		if err := g.setupRulesFile(packageDir, cfg.Settings.RulesFile); err != nil {
			return fmt.Errorf("failed to setup rules file: %w", err)
		}
	}

	// 3. Build context using `cx`
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.BuildContext(packageDir); err != nil {
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
		// Handle different generation types
		if section.Type == "schema_to_md" {
			if err := g.generateFromSchema(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Schema to Markdown generation failed for section '%s'", section.Name)
			}
			continue
		}
		if section.Type == "schema_table" {
			if err := g.generateFromSchemaTable(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Schema table generation failed for section '%s'", section.Name)
			}
			continue
		}
		if section.Type == "schema_describe" {
			if err := g.generateSchemaDescriptions(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Schema descriptions generation failed for section '%s'", section.Name)
			}
			continue
		}
		if section.Type == "doc_sections" {
			if err := g.generateFromDocSections(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Doc sections generation failed for section '%s'", section.Name)
			}
			continue
		}
		if section.Type == "capture" {
			if err := g.generateFromCapture(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("CLI capture generation failed for section '%s'", section.Name)
			}
			continue
		}
		if section.Type == "nb_concept" {
			if err := g.generateFromConcept(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Concept generation failed for section '%s'", section.Name)
			}
			continue
		}
		g.logger.Infof("Generating section: %s", section.Name)

		// Use the new prompt resolution method that checks notebook first
		promptContent, err := g.resolvePromptContent(packageDir, section.Prompt)
		if err != nil {
			return fmt.Errorf("could not resolve prompt for section '%s': %w", section.Name, err)
		}

		// Build the final prompt with system prompt prepended if available
		finalPrompt := string(promptContent)
		if systemPrompt != "" {
			finalPrompt = systemPrompt + "\n" + finalPrompt
		}

		// Handle reference mode
		if cfg.Settings.RegenerationMode == "reference" {
			existingOutputPath := filepath.Join(outputBaseDir, section.Output)
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

		output, err := g.CallLLM(finalPrompt, model, genConfig, packageDir)
		if err != nil {
			g.logger.WithError(err).Errorf("LLM call failed for section '%s'", section.Name)
			continue // Continue to the next section even if one fails
		}

		// 6. Write output to the determined output directory
		outputPath := filepath.Join(outputBaseDir, section.Output)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write section output: %w", err)
		}
		g.logger.Infof("Successfully wrote section '%s' to %s", section.Name, outputPath)
		ulog.Success("Wrote section").
			Field("section", section.Name).
			Field("path", outputPath).
			Emit()
	}

	return nil
}

const SchemaToMarkdownSystemPrompt = `You are a technical writer tasked with creating documentation from one or more JSON schemas.
Convert the provided plain text descriptions of JSON schemas into a user-friendly Markdown document.

**Instructions:**
- Create a clear, well-structured document.
- The document will likely contain multiple schema sections.
- For each "NEW SCHEMA SECTION" provided in the input:
  - Create a Level 2 Heading (##) using the provided "Schema Section Title".
  - Use a simple two-column Markdown table: Property and Description.
  - In the Description column, start with inline metadata in parentheses: (type, required/optional, default: value if any). For example: "(string, required)" or "(integer, optional, default: 3)".
  - After the metadata, write a verbose, helpful explanation that goes beyond the schema's terse descriptions. Explain what the property does, when you might use it, and any important considerations.
  - For nested objects, use Level 3 sub-headings (###) and separate tables.
  - Immediately after each H2 section's property table, include a small example TOML code block (no heading needed) showing a brief, realistic configuration snippet for that section. Keep it concise - just 3-5 lines demonstrating the key properties.
- Do not include any preamble or explanation about your process. Your output should be only the final Markdown document.

**Status Badges (IMPORTANT):**
At the END of each property's description, append HTML badges based on the schema metadata:
- If "Status: ALPHA" → append: <span class="schema-badge schema-badge-alpha">ALPHA</span>
- If "Status: BETA" → append: <span class="schema-badge schema-badge-beta">BETA</span>
- If "Status: DEPRECATED" or "Deprecated: true" → append: <span class="schema-badge schema-badge-deprecated">DEPRECATED</span>
- If there's a "Notice:" → append it in muted style: <span class="schema-status-msg">notice text</span>
- If there's a "Replaced By:" → append: <span class="schema-status-msg">→ <code>replacement</code></span>
- If "Wizard: true" → prefix the property name with ★ in the table

Example description with badges:
"(object, optional) Settings for embedded Neovim. <span class="schema-badge schema-badge-alpha">ALPHA</span> <span class="schema-status-msg">Experimental feature</span>"
---
`

const DocSectionsSystemPrompt = `You are creating a configuration reference document with multiple sections.
Each section represents a different configuration context (e.g., user config, ecosystem config, package config).

**Input Format:**
Each "SECTION" in the input includes:
- Title: The H2 heading to use
- Description: What this configuration context is for
- Properties: Which properties to document and include in the example
- Documentation: The source docs to pull descriptions from

**Output Format (Markdown):**

For EACH section provided, create:

## [Section Title]

[Section Description - one paragraph explaining when/where this config is used]

| Property | Description |
|----------|-------------|
[Table rows for each property listed, with descriptions VERBATIM from the source docs]

` + "```toml" + `
# [Brief comment about this config context]
[Realistic example using ONLY the properties listed for this section]
[Include inline comments with descriptions from the docs]
` + "```" + `

**Rules:**
- Create one H2 section for each input section
- Use exact wording from the docs for descriptions - do not paraphrase
- Each section gets its own TOML example with only that section's properties
- All TOML must be inside fenced code blocks
- No preamble or explanation outside the specified format
---
`

func (g *Generator) generateFromSchema(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating section from schema: %s", section.Name)

	// Normalize inputs: either multiple Schemas or single Source
	var inputs []config.SchemaInput
	if len(section.Schemas) > 0 {
		inputs = section.Schemas
	} else if section.Source != "" {
		inputs = []config.SchemaInput{{Path: section.Source}}
	} else {
		return fmt.Errorf("section type 'schema_to_md' requires 'schemas' list or 'source' file")
	}

	var sb strings.Builder

	for _, input := range inputs {
		if input.Path == "" {
			continue
		}

		schemaPath := filepath.Join(packageDir, input.Path)
		parser, err := schema.NewParser(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to initialize schema parser for %s: %w", input.Path, err)
		}

		schemaText, err := parser.RenderAsText()
		if err != nil {
			return fmt.Errorf("failed to render schema %s as text: %w", input.Path, err)
		}

		sb.WriteString("\n--- NEW SCHEMA SECTION ---\n")
		if input.Title != "" {
			sb.WriteString(fmt.Sprintf("Schema Section Title: %s\n", input.Title))
		}
		sb.WriteString(fmt.Sprintf("Source File: %s\n", input.Path))
		sb.WriteString(schemaText)
		sb.WriteString("\n")
	}

	finalPrompt := SchemaToMarkdownSystemPrompt + sb.String()

	// Handle reference mode - inject existing output for LLM to update rather than regenerate
	outputPath := filepath.Join(outputBaseDir, section.Output)
	if cfg.Settings.RegenerationMode == "reference" {
		if existingDocs, err := os.ReadFile(outputPath); err == nil {
			g.logger.Debugf("Injecting reference content from %s", outputPath)
			finalPrompt = "For your reference, here is the previous version of the documentation. Preserve any manual edits while updating with new schema information:\n\n<reference_docs>\n" +
				string(existingDocs) + "\n</reference_docs>\n\n---\n\n" + finalPrompt
		}
	}

	// Determine model to use (section override or global)
	model := cfg.Settings.Model
	if section.Model != "" {
		model = section.Model
	}

	genConfig := config.MergeGenerationConfig(cfg.Settings.GenerationConfig, section.GenerationConfig)

	output, err := g.CallLLM(finalPrompt, model, genConfig, packageDir)
	if err != nil {
		return fmt.Errorf("LLM call failed for schema section '%s': %w", section.Name, err)
	}

	// Write to the determined output directory
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory for schema doc: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write schema doc output: %w", err)
	}
	g.logger.Infof("Successfully wrote schema doc section '%s' to %s", section.Name, outputPath)
	return nil
}

func (g *Generator) generateFromDocSections(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating doc sections: %s", section.Name)

	if len(section.DocSources) == 0 {
		return fmt.Errorf("section type 'doc_sections' requires 'doc_sources' list")
	}

	// Discover all ecosystems to build package path map
	discoveryService := workspace.NewDiscoveryService(g.logger)
	result, err := discoveryService.DiscoverAll()
	if err != nil {
		return fmt.Errorf("failed to discover ecosystems: %w", err)
	}

	// Build map of package name -> path
	packagePaths := make(map[string]string)
	for _, eco := range result.Ecosystems {
		configPath, err := coreConfig.FindConfigFile(eco.Path)
		if err != nil {
			continue
		}
		ecoCfg, err := coreConfig.Load(configPath)
		if err != nil {
			continue
		}

		for _, wsPattern := range ecoCfg.Workspaces {
			pattern := filepath.Join(eco.Path, wsPattern)
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err == nil && info.IsDir() {
					pkgName := filepath.Base(match)
					packagePaths[pkgName] = match
				}
			}
		}
	}

	// Build combined content from doc sections
	var sb strings.Builder

	for _, source := range section.DocSources {
		pkgPath, ok := packagePaths[source.Package]
		if !ok {
			return fmt.Errorf("package '%s' not found in configured ecosystems", source.Package)
		}

		// Auto-discover schema doc if not specified
		docFile := source.Doc
		if docFile == "" {
			docFile = g.findSchemaDoc(pkgPath)
			if docFile == "" {
				return fmt.Errorf("could not auto-discover schema doc for package '%s' (no schema_to_md section found)", source.Package)
			}
			g.logger.Debugf("Auto-discovered schema doc for %s: %s", source.Package, docFile)
		}

		// Try notebook docgen dir first, then package docs/
		docPath := g.resolveDocPath(pkgPath, docFile)
		if docPath == "" {
			return fmt.Errorf("doc file '%s' not found for package '%s'", docFile, source.Package)
		}

		content, err := os.ReadFile(docPath)
		if err != nil {
			return fmt.Errorf("failed to read doc %s: %w", docPath, err)
		}

		// Add section info and content
		sb.WriteString("\n--- SECTION ---\n")
		if source.Title != "" {
			sb.WriteString(fmt.Sprintf("Title: %s\n", source.Title))
		} else {
			sb.WriteString(fmt.Sprintf("Title: %s Configuration\n", source.Package))
		}
		if source.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", source.Description))
		}
		sb.WriteString(fmt.Sprintf("Properties: %v\n", source.Properties))
		sb.WriteString(fmt.Sprintf("Package: %s\n\n", source.Package))
		sb.WriteString("--- SOURCE DOCUMENTATION ---\n")
		sb.WriteString(string(content))
		sb.WriteString("\n--- END SOURCE ---\n\n")
	}

	// Send to LLM to add unified example
	finalPrompt := DocSectionsSystemPrompt + "\n--- DOCUMENTATION SECTIONS ---\n\n" + sb.String()

	// Handle reference mode - inject existing output for LLM to update rather than regenerate
	outputPath := filepath.Join(outputBaseDir, section.Output)
	if cfg.Settings.RegenerationMode == "reference" {
		if existingDocs, err := os.ReadFile(outputPath); err == nil {
			g.logger.Debugf("Injecting reference content from %s", outputPath)
			finalPrompt = "For your reference, here is the previous version of the documentation. Preserve any manual edits while updating with new information:\n\n<reference_docs>\n" +
				string(existingDocs) + "\n</reference_docs>\n\n---\n\n" + finalPrompt
		}
	}

	model := cfg.Settings.Model
	if section.Model != "" {
		model = section.Model
	}

	genConfig := config.MergeGenerationConfig(cfg.Settings.GenerationConfig, section.GenerationConfig)

	output, err := g.CallLLM(finalPrompt, model, genConfig, packageDir)
	if err != nil {
		return fmt.Errorf("LLM call failed for doc sections '%s': %w", section.Name, err)
	}

	// Write output
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write doc sections output: %w", err)
	}
	g.logger.Infof("Successfully wrote doc sections '%s' to %s", section.Name, outputPath)
	return nil
}

// findSchemaDoc looks for a schema_to_md section in the package's docgen config and returns its output path.
func (g *Generator) findSchemaDoc(pkgPath string) string {
	// Try to load the package's docgen config
	cfg, _, err := config.LoadWithNotebook(pkgPath)
	if err != nil {
		return ""
	}

	// Find the first schema_to_md section
	for _, section := range cfg.Sections {
		if section.Type == "schema_to_md" {
			return "docs/" + section.Output
		}
	}

	return ""
}

// resolveDocPath finds the doc file, trying notebook location first then package docs/
func (g *Generator) resolveDocPath(pkgPath, docFile string) string {
	// Try notebook docgen/docs/ first
	node, err := workspace.GetProjectByPath(pkgPath)
	if err == nil {
		cfg, err := coreConfig.LoadDefault()
		if err == nil {
			locator := workspace.NewNotebookLocator(cfg)
			docgenDir, err := locator.GetDocgenDir(node)
			if err == nil {
				notebookPath := filepath.Join(docgenDir, docFile)
				if _, err := os.Stat(notebookPath); err == nil {
					return notebookPath
				}
			}
		}
	}

	// Fallback to package path
	pkgDocPath := filepath.Join(pkgPath, docFile)
	if _, err := os.Stat(pkgDocPath); err == nil {
		return pkgDocPath
	}

	return ""
}

func (g *Generator) generateFromSchemaTable(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating schema table: %s", section.Name)

	// Check for JSON format - dispatch to JSON generator
	if section.Format == "json" {
		return g.generateFromSchemaTableJSON(packageDir, section, cfg, outputBaseDir)
	}

	// Normalize inputs: either multiple Schemas or single Source
	var inputs []config.SchemaInput
	if len(section.Schemas) > 0 {
		inputs = section.Schemas
	} else if section.Source != "" {
		inputs = []config.SchemaInput{{Path: section.Source}}
	} else {
		return fmt.Errorf("section type 'schema_table' requires 'schemas' list or 'source' file")
	}

	// Load descriptions if configured
	var descriptions map[string]string
	if section.Descriptions != "" {
		var err error
		descriptions, err = g.loadDescriptions(packageDir, outputBaseDir, section.Descriptions)
		if err != nil {
			g.logger.WithError(err).Warnf("Could not load descriptions file, using schema descriptions")
		} else {
			g.logger.Infof("Loaded %d descriptions from %s", len(descriptions), section.Descriptions)
		}
	}

	var sb strings.Builder

	// Add title
	sb.WriteString(fmt.Sprintf("# %s\n\n", section.Title))

	for _, input := range inputs {
		if input.Path == "" {
			continue
		}

		schemaPath := filepath.Join(packageDir, input.Path)
		p, err := schema.NewParser(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to initialize schema parser for %s: %w", input.Path, err)
		}

		props, err := p.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse schema %s: %w", input.Path, err)
		}

		if input.Title != "" {
			sb.WriteString(fmt.Sprintf("## %s\n\n", input.Title))
		}

		// Generate the table with layer column
		sb.WriteString("| Property | Type | Layer | Description |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- |\n")

		for _, prop := range props {
			g.writeSchemaTableRow(&sb, prop, "", descriptions)
		}
		sb.WriteString("\n")
	}

	// Write output
	outputPath := filepath.Join(outputBaseDir, section.Output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write schema table output: %w", err)
	}

	g.logger.Infof("Successfully wrote schema table '%s' to %s", section.Name, outputPath)
	return nil
}

// ConfigNode represents a configuration property in the JSON output for the website.
// It preserves the hierarchical structure and includes all metadata for rich UI rendering.
type ConfigNode struct {
	Name             string       `json:"name"`
	Path             string       `json:"path"`                       // Full dotted path (e.g., "groves.mygrove.enabled")
	Type             string       `json:"type"`
	Description      string       `json:"description"`
	Required         bool         `json:"required,omitempty"`
	Default          interface{}  `json:"default,omitempty"`
	Deprecated       bool         `json:"deprecated,omitempty"`
	Layer            string       `json:"layer,omitempty"`            // global, ecosystem, project
	Priority         int          `json:"priority,omitempty"`
	Wizard           bool         `json:"wizard,omitempty"`           // Common setup field (★)
	Hint             string       `json:"hint,omitempty"`
	Status           string       `json:"status,omitempty"`           // alpha, beta, stable, deprecated
	StatusMessage    string       `json:"statusMessage,omitempty"`
	StatusReplacedBy string       `json:"statusReplacedBy,omitempty"`
	Children         []ConfigNode `json:"children,omitempty"`
}

// ConfigSchemaJSON is the root structure for the JSON output
type ConfigSchemaJSON struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Properties  []ConfigNode `json:"properties"`
}

// generateFromSchemaTableJSON outputs the schema as structured JSON for the website component
func (g *Generator) generateFromSchemaTableJSON(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating schema table (JSON format): %s", section.Name)

	// Normalize inputs: either multiple Schemas or single Source
	var inputs []config.SchemaInput
	if len(section.Schemas) > 0 {
		inputs = section.Schemas
	} else if section.Source != "" {
		inputs = []config.SchemaInput{{Path: section.Source}}
	} else {
		return fmt.Errorf("section type 'schema_table' requires 'schemas' list or 'source' file")
	}

	// Load descriptions if configured
	var descriptions map[string]string
	if section.Descriptions != "" {
		var err error
		descriptions, err = g.loadDescriptions(packageDir, outputBaseDir, section.Descriptions)
		if err != nil {
			g.logger.WithError(err).Warnf("Could not load descriptions file, using schema descriptions")
		} else {
			g.logger.Infof("Loaded %d descriptions from %s", len(descriptions), section.Descriptions)
		}
	}

	// Build the JSON structure
	result := ConfigSchemaJSON{
		Title:      section.Title,
		Properties: []ConfigNode{},
	}

	for _, input := range inputs {
		if input.Path == "" {
			continue
		}

		schemaPath := filepath.Join(packageDir, input.Path)
		p, err := schema.NewParser(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to initialize schema parser for %s: %w", input.Path, err)
		}

		props, err := p.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse schema %s: %w", input.Path, err)
		}

		// Convert schema properties to ConfigNodes
		nodes := g.schemaPropsToConfigNodes(props, "", descriptions)
		result.Properties = append(result.Properties, nodes...)
	}

	// Determine output paths
	// If output ends with .md, we create both .json (data) and .md (wrapper)
	// If output ends with .json, we only create the JSON file
	mdOutput := section.Output
	var jsonOutput string

	if strings.HasSuffix(section.Output, ".md") {
		// Replace .md with .json for the data file
		jsonOutput = strings.TrimSuffix(section.Output, ".md") + ".json"
	} else if strings.HasSuffix(section.Output, ".json") {
		jsonOutput = section.Output
		mdOutput = "" // No markdown wrapper needed
	} else {
		// Default: add .json suffix
		jsonOutput = section.Output + ".json"
	}

	// Create output directory
	if err := os.MkdirAll(outputBaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write JSON data file
	jsonPath := filepath.Join(outputBaseDir, jsonOutput)
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config schema to JSON: %w", err)
	}

	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write schema table JSON output: %w", err)
	}
	g.logger.Infof("Successfully wrote schema table JSON '%s' to %s", section.Name, jsonPath)

	// Write markdown wrapper file with config-reference code block
	if mdOutput != "" {
		// The JSON will be served from /data/{package}/{filename}.json
		// Use the package directory name (workspace name) as the package identifier
		packageName := filepath.Base(packageDir)

		mdContent := fmt.Sprintf(`# %s

`+"```config-reference"+`
{"src": "/data/%s/%s"}
`+"```"+`
`, section.Title, packageName, jsonOutput)

		mdPath := filepath.Join(outputBaseDir, mdOutput)
		if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
			return fmt.Errorf("failed to write schema table markdown wrapper: %w", err)
		}
		g.logger.Infof("Successfully wrote schema table markdown wrapper to %s", mdPath)
	}

	return nil
}

// schemaPropsToConfigNodes converts schema.Property slice to ConfigNode slice
func (g *Generator) schemaPropsToConfigNodes(props []schema.Property, prefix string, descriptions map[string]string) []ConfigNode {
	var nodes []ConfigNode

	for _, prop := range props {
		// Build full path
		path := prop.Name
		if prefix != "" {
			path = prefix + "." + prop.Name
		}

		// Get description - prefer LLM-generated description
		desc := prop.Description
		if descriptions != nil {
			if llmDesc, ok := descriptions[path]; ok && llmDesc != "" {
				desc = llmDesc
			}
		}

		node := ConfigNode{
			Name:             prop.Name,
			Path:             path,
			Type:             prop.Type,
			Description:      desc,
			Required:         prop.Required,
			Default:          prop.Default,
			Deprecated:       prop.Deprecated,
			Layer:            prop.Layer,
			Priority:         prop.Priority,
			Wizard:           prop.Wizard,
			Hint:             prop.Hint,
			Status:           prop.Status,
			StatusMessage:    prop.StatusMessage,
			StatusReplacedBy: prop.StatusReplacedBy,
		}

		// Recursively process children
		if len(prop.Properties) > 0 {
			node.Children = g.schemaPropsToConfigNodes(prop.Properties, path, descriptions)
		}

		nodes = append(nodes, node)
	}

	return nodes
}

// writeSchemaTableRow writes a single property row to the schema table, including nested properties
// If descriptions map is provided, it will use LLM-generated descriptions instead of schema descriptions
func (g *Generator) writeSchemaTableRow(sb *strings.Builder, prop schema.Property, prefix string, descriptions map[string]string) {
	// Build property name with prefix for nested fields
	propName := prop.Name
	if prefix != "" {
		propName = prefix + "." + prop.Name
	}

	// Name column: wizard star, name, deprecated strikethrough
	name := fmt.Sprintf("`%s`", propName)
	if prop.Wizard {
		name = "★ " + name
	}
	if prop.Deprecated {
		name = "~~" + name + "~~"
	}

	// Layer column with badge-style formatting
	layer := ""
	if prop.Layer != "" {
		layer = fmt.Sprintf("**%s**", strings.Title(prop.Layer))
	}

	// Build description with metadata
	var descParts []string

	// Main description - use LLM description if available, otherwise schema description
	mainDesc := prop.Description
	if descriptions != nil {
		if llmDesc, ok := descriptions[propName]; ok && llmDesc != "" {
			mainDesc = llmDesc
		}
	}
	if mainDesc != "" {
		descParts = append(descParts, mainDesc)
	}

	// Status badges as styled HTML spans - always show badge, skip redundant message
	descLower := strings.ToLower(mainDesc)
	if prop.Status != "" && prop.Status != "stable" {
		statusBadge := fmt.Sprintf(`<span class="schema-badge schema-badge-%s">%s</span>`, prop.Status, strings.ToUpper(prop.Status))
		// Only add message if description doesn't already mention the replacement or similar info
		replacementMentioned := prop.StatusReplacedBy != "" && strings.Contains(descLower, strings.ToLower(prop.StatusReplacedBy))
		if prop.StatusMessage != "" && !replacementMentioned {
			statusBadge += fmt.Sprintf(` <span class="schema-status-msg">%s</span>`, prop.StatusMessage)
		}
		descParts = append(descParts, statusBadge)
	}

	// Replacement hint with code styling - skip if already in description
	if prop.StatusReplacedBy != "" && !strings.Contains(descLower, strings.ToLower(prop.StatusReplacedBy)) {
		descParts = append(descParts, fmt.Sprintf(`<span class="schema-status-msg">→ <code>%s</code></span>`, prop.StatusReplacedBy))
	}

	// Hint
	if prop.Hint != "" {
		descParts = append(descParts, fmt.Sprintf("_Hint: %s_", prop.Hint))
	}

	// Default value
	if prop.Default != nil {
		descParts = append(descParts, fmt.Sprintf("Default: `%v`", prop.Default))
	}

	// Required indicator
	if prop.Required {
		descParts = append(descParts, "**Required**")
	}

	desc := strings.Join(descParts, " · ")
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.ReplaceAll(desc, "|", "\\|") // Escape pipes for markdown tables

	sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", name, prop.Type, layer, desc))

	// Write nested properties with indented prefix
	for _, child := range prop.Properties {
		g.writeSchemaTableRow(sb, child, propName, descriptions)
	}
}

// generateSchemaDescriptions uses LLM to generate rich descriptions for schema properties
// and saves them to a JSON file that can be used by schema_table.
func (g *Generator) generateSchemaDescriptions(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating schema descriptions: %s", section.Name)

	// Normalize inputs
	var inputs []config.SchemaInput
	if len(section.Schemas) > 0 {
		inputs = section.Schemas
	} else if section.Source != "" {
		inputs = []config.SchemaInput{{Path: section.Source}}
	} else {
		return fmt.Errorf("section type 'schema_describe' requires 'schemas' list or 'source' file")
	}

	// Collect all properties from all schemas
	var allProps []schema.Property
	for _, input := range inputs {
		if input.Path == "" {
			continue
		}
		schemaPath := filepath.Join(packageDir, input.Path)
		p, err := schema.NewParser(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to parse schema %s: %w", input.Path, err)
		}
		props, err := p.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse schema %s: %w", input.Path, err)
		}
		allProps = append(allProps, props...)
	}

	// Build prompt for LLM
	var promptBuilder strings.Builder
	promptBuilder.WriteString(`Generate detailed, helpful descriptions for each configuration property below.
Output JSON with property paths as keys and description strings as values.
Each description should:
- Explain what the property does
- When/why you would use it
- Any important considerations
Keep descriptions concise but informative (1-3 sentences).

Properties to describe:
`)
	g.collectPropertyPaths(&promptBuilder, allProps, "")

	promptBuilder.WriteString(`
Output format (JSON only, no markdown fences):
{
  "property.path": "Description text here...",
  ...
}`)

	// Call LLM
	model := section.Model
	if model == "" {
		model = cfg.Settings.Model
	}
	if model == "" {
		model = "gemini-2.0-flash"
	}

	genConfig := config.MergeGenerationConfig(cfg.Settings.GenerationConfig, section.GenerationConfig)
	response, err := g.CallLLM(promptBuilder.String(), model, genConfig, packageDir)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse and validate JSON response
	var descriptions map[string]string
	// Strip markdown code fences if present
	cleanResponse := strings.TrimSpace(response)
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	if err := json.Unmarshal([]byte(cleanResponse), &descriptions); err != nil {
		return fmt.Errorf("failed to parse LLM response as JSON: %w\nResponse: %s", err, response)
	}

	// Write output
	outputPath := filepath.Join(outputBaseDir, section.Output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(descriptions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal descriptions: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write descriptions file: %w", err)
	}

	g.logger.Infof("Successfully wrote %d descriptions to %s", len(descriptions), outputPath)
	return nil
}

// collectPropertyPaths recursively collects property paths for the LLM prompt
func (g *Generator) collectPropertyPaths(sb *strings.Builder, props []schema.Property, prefix string) {
	for _, prop := range props {
		path := prop.Name
		if prefix != "" {
			path = prefix + "." + prop.Name
		}
		sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", path, prop.Type, prop.Description))

		// Recurse into nested properties
		if len(prop.Properties) > 0 {
			g.collectPropertyPaths(sb, prop.Properties, path)
		}
	}
}

// loadDescriptions loads LLM-generated descriptions from a JSON file
// It checks outputBaseDir first (for notebook mode), then packageDir
func (g *Generator) loadDescriptions(packageDir, outputBaseDir, descriptionsPath string) (map[string]string, error) {
	if descriptionsPath == "" {
		return nil, nil
	}

	// Try outputBaseDir first (where schema_describe writes to)
	fullPath := filepath.Join(outputBaseDir, filepath.Base(descriptionsPath))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		// Fall back to packageDir-relative path
		fullPath = filepath.Join(packageDir, descriptionsPath)
		data, err = os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read descriptions file (tried %s and %s): %w",
				filepath.Join(outputBaseDir, filepath.Base(descriptionsPath)),
				filepath.Join(packageDir, descriptionsPath), err)
		}
	}

	var descriptions map[string]string
	if err := json.Unmarshal(data, &descriptions); err != nil {
		return nil, fmt.Errorf("failed to parse descriptions file: %w", err)
	}

	g.logger.Debugf("Loaded descriptions from %s", fullPath)
	return descriptions, nil
}

func (g *Generator) generateFromCapture(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating CLI capture section: %s", section.Name)

	if section.Binary == "" {
		return fmt.Errorf("section type 'capture' requires 'binary' (binary name)")
	}

	// Determine output format (default to styled)
	format := capture.FormatHTML
	if section.Format == "plain" {
		format = capture.FormatMarkdown
	}

	// Determine recursion depth (default to 5)
	depth := 5
	if section.Depth > 0 {
		depth = section.Depth
	}

	// Create capturer and run
	capturer := capture.New(g.logger)
	opts := capture.Options{
		MaxDepth:        depth,
		Format:          format,
		SubcommandOrder: section.SubcommandOrder,
	}

	outputPath := filepath.Join(outputBaseDir, section.Output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory for capture: %w", err)
	}

	if err := capturer.Capture(section.Binary, outputPath, opts); err != nil {
		return fmt.Errorf("CLI capture failed for section '%s': %w", section.Name, err)
	}

	g.logger.Infof("Successfully captured CLI reference for '%s' to %s", section.Binary, outputPath)
	return nil
}

func (g *Generator) setupRulesFile(packageDir, rulesFile string) error {
	// Read the specified rules file
	// If the rules file path starts with .cx/ or is an absolute path, use it directly
	// Otherwise, look for it in the docs/ directory (legacy behavior)
	var rulesPath string
	if strings.HasPrefix(rulesFile, ".cx/") || filepath.IsAbs(rulesFile) {
		rulesPath = filepath.Join(packageDir, rulesFile)
	} else {
		rulesPath = filepath.Join(packageDir, "docs", rulesFile)
	}

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

// BuildContext runs cx generate to prepare context for LLM calls
func (g *Generator) BuildContext(packageDir string) error {
	// Use 'grove cx generate' for workspace-awareness
	cmd := delegation.Command("cx", "generate")
	cmd.Dir = packageDir
	// Discard output to avoid contaminating the LLM response
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// CallLLM makes an LLM request with the given prompt and configuration
func (g *Generator) CallLLM(promptContent, model string, genConfig config.GenerationConfig, workDir string) (string, error) {
	// Use provided model or default to gemini-2.0-flash
	if model == "" {
		model = "gemini-2.0-flash"
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
		"--file", promptFile.Name(),
		"--model", model,
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

	cmd := delegation.Command(args[0], args[1:]...)
	cmd.Dir = workDir

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// Log the stderr for debugging
		g.logger.Debugf("LLM stderr: %s", stderr.String())
		return "", fmt.Errorf("grove llm request failed: %w", err)
	}

	// Try stdout first, which should now have the content
	// (after fixing grove-gemini to output to stdout)
	output := stdout.Bytes()
	if len(output) == 0 {
		// Fallback to stderr for backward compatibility
		// (in case older version of grove llm is being used)
		output = stderr.Bytes()

		// If we're using stderr, we need to extract just the content
		// The stderr contains logs + token usage box + actual content
		// Find the last occurrence of the token usage box closing
		stderrStr := string(output)

		// Look for the end of the token usage box: "╰──────────────────────────────────╯"
		boxEnd := strings.LastIndex(stderrStr, "╰──────────────────────────────────╯")
		if boxEnd != -1 {
			// Content starts after the box
			content := stderrStr[boxEnd+len("╰──────────────────────────────────╯"):]
			output = []byte(strings.TrimSpace(content))
		}
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

// generateSectionsMode handles output_mode: sections where the top-level config
// is a website content aggregator. Sections live in subdirectories (e.g., overview/,
// concepts/), each with their own docgen.config.yml. This method discovers those
// subdirectory configs, merges their sections, and generates from each.
func (g *Generator) generateSectionsMode(packageDir, configPath string, topCfg *config.DocgenConfig, opts GenerateOptions) error {
	docgenDir := filepath.Dir(configPath)
	g.logger.Infof("Sections mode: scanning subdirectories in %s", docgenDir)
	ulog.Info("Sections mode").
		Field("docgenDir", docgenDir).
		Emit()

	// Build context once for the whole package
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.BuildContext(packageDir); err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	// Discover subdirectories with their own docgen.config.yml
	type subSection struct {
		subDir    string              // subdirectory path (e.g., .../docgen/overview)
		subCfg    *config.DocgenConfig
		section   config.SectionConfig
	}

	var allSections []subSection

	entries, err := os.ReadDir(docgenDir)
	if err != nil {
		return fmt.Errorf("failed to read docgen directory %s: %w", docgenDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subDirPath := filepath.Join(docgenDir, entry.Name())
		subConfigPath := filepath.Join(subDirPath, config.ConfigFileName)

		if _, statErr := os.Stat(subConfigPath); os.IsNotExist(statErr) {
			continue
		}

		subCfg, loadErr := config.LoadFromPath(subConfigPath)
		if loadErr != nil {
			g.logger.Warnf("Failed to load config from %s: %v", subConfigPath, loadErr)
			continue
		}

		g.logger.Infof("Found section directory: %s (%d sections)", entry.Name(), len(subCfg.Sections))

		for _, section := range subCfg.Sections {
			allSections = append(allSections, subSection{
				subDir:  subDirPath,
				subCfg:  subCfg,
				section: section,
			})
		}
	}

	if len(allSections) == 0 {
		return fmt.Errorf("no section subdirectories with %s found in %s", config.ConfigFileName, docgenDir)
	}

	// Build a list of qualified names (subdir/section) for display and lookup
	qualifiedName := func(ss subSection) string {
		return filepath.Base(ss.subDir) + "/" + ss.section.Name
	}

	// Filter sections if specific ones were requested
	// Supports both bare names ("introduction") and namespaced ("overview/introduction").
	// Bare names work if unique; if ambiguous, an error lists the namespaced alternatives.
	sectionsToGenerate := allSections
	if len(opts.Sections) > 0 {
		var filtered []subSection
		var errors []string

		for _, requested := range opts.Sections {
			if strings.Contains(requested, "/") {
				// Namespaced: match exactly against subdir/name
				var found bool
				for _, ss := range allSections {
					if qualifiedName(ss) == requested {
						filtered = append(filtered, ss)
						found = true
						break
					}
				}
				if !found {
					var available []string
					for _, ss := range allSections {
						available = append(available, qualifiedName(ss))
					}
					errors = append(errors, fmt.Sprintf("section %q not found (available: %v)", requested, available))
				}
			} else {
				// Bare name: find all matches across subdirectories
				var matches []subSection
				for _, ss := range allSections {
					if ss.section.Name == requested {
						matches = append(matches, ss)
					}
				}
				switch len(matches) {
				case 0:
					var available []string
					for _, ss := range allSections {
						available = append(available, qualifiedName(ss))
					}
					errors = append(errors, fmt.Sprintf("section %q not found (available: %v)", requested, available))
				case 1:
					filtered = append(filtered, matches[0])
				default:
					var ambiguous []string
					for _, m := range matches {
						ambiguous = append(ambiguous, qualifiedName(m))
					}
					errors = append(errors, fmt.Sprintf("section %q is ambiguous, use a qualified name: %v", requested, ambiguous))
				}
			}
		}

		if len(errors) > 0 {
			return fmt.Errorf("%s", strings.Join(errors, "; "))
		}

		sectionsToGenerate = filtered
		g.logger.Infof("Generating %d of %d sections: %v", len(sectionsToGenerate), len(allSections), opts.Sections)
	}

	// Generate each section using its subdirectory context
	for _, ss := range sectionsToGenerate {
		g.logger.Infof("Generating section: %s", qualifiedName(ss))

		// Determine output directory for this section
		outputDir := filepath.Join(ss.subDir, "docs")
		if ss.subCfg.Settings.OutputDir != "" {
			outputDir = filepath.Join(ss.subDir, ss.subCfg.Settings.OutputDir)
		}

		// Handle special section types that don't use prompt files
		if ss.section.Type == "schema_to_md" {
			if err := g.generateFromSchema(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Schema to Markdown generation failed for section '%s'", ss.section.Name)
			}
			continue
		}
		if ss.section.Type == "schema_table" {
			if err := g.generateFromSchemaTable(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Schema table generation failed for section '%s'", ss.section.Name)
			}
			continue
		}
		if ss.section.Type == "schema_describe" {
			if err := g.generateSchemaDescriptions(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Schema descriptions generation failed for section '%s'", ss.section.Name)
			}
			continue
		}
		if ss.section.Type == "doc_sections" {
			if err := g.generateFromDocSections(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Doc sections generation failed for section '%s'", ss.section.Name)
			}
			continue
		}
		if ss.section.Type == "capture" {
			if err := g.generateFromCapture(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("CLI capture generation failed for section '%s'", ss.section.Name)
			}
			continue
		}

		// Standard prompt-based generation
		// Resolve prompt from the subdirectory's prompts/ folder
		promptPath := filepath.Join(ss.subDir, "prompts", ss.section.Prompt)
		promptContent, err := os.ReadFile(promptPath)
		if err != nil {
			return fmt.Errorf("could not read prompt for section '%s' at %s: %w", ss.section.Name, promptPath, err)
		}

		// Build the final prompt with system prompt if configured
		finalPrompt := string(promptContent)
		if ss.subCfg.Settings.SystemPrompt != "" {
			if ss.subCfg.Settings.SystemPrompt == "default" {
				finalPrompt = DefaultSystemPrompt + "\n" + finalPrompt
			} else {
				systemPromptPath := filepath.Join(ss.subDir, ss.subCfg.Settings.SystemPrompt)
				if content, readErr := os.ReadFile(systemPromptPath); readErr == nil {
					finalPrompt = string(content) + "\n" + finalPrompt
				}
			}
		}

		// Handle reference mode
		if ss.subCfg.Settings.RegenerationMode == "reference" {
			existingOutputPath := filepath.Join(outputDir, ss.section.Output)
			if existingDocs, readErr := os.ReadFile(existingOutputPath); readErr == nil {
				g.logger.Debugf("Injecting reference content from %s", existingOutputPath)
				finalPrompt = "For your reference, here is the previous version of the documentation:\n\n<reference_docs>\n" +
					string(existingDocs) + "\n</reference_docs>\n\n---\n\n" + finalPrompt
			}
		}

		// Determine model (section override > sub-config > top-level)
		model := topCfg.Settings.Model
		if ss.subCfg.Settings.Model != "" {
			model = ss.subCfg.Settings.Model
		}
		if ss.section.Model != "" {
			model = ss.section.Model
		}

		genConfig := config.MergeGenerationConfig(ss.subCfg.Settings.GenerationConfig, ss.section.GenerationConfig)

		output, err := g.CallLLM(finalPrompt, model, genConfig, packageDir)
		if err != nil {
			g.logger.WithError(err).Errorf("LLM call failed for section '%s'", ss.section.Name)
			continue
		}

		// Write output to the subdirectory's docs/ folder
		outputPath := filepath.Join(outputDir, ss.section.Output)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write section output: %w", err)
		}
		g.logger.Infof("Successfully wrote section '%s' to %s", ss.section.Name, outputPath)
		ulog.Success("Wrote section").
			Field("section", ss.section.Name).
			Field("path", outputPath).
			Emit()
	}

	return nil
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