package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/logging"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/core/util/delegation"
	"github.com/grovetools/docgen/pkg/capture"
	"github.com/grovetools/docgen/pkg/config"
	"github.com/grovetools/docgen/pkg/parser"
	"github.com/grovetools/docgen/pkg/schema"
	"github.com/grovetools/grove-anthropic/pkg/anthropic"
	"github.com/sirupsen/logrus"
)

var ulog = logging.NewUnifiedLogger("grove-docgen")

// Generator handles the documentation generation for a single package.
type Generator struct {
	logger *logrus.Logger

	// Fan-out state, populated per run when the effective model is a Claude
	// model (see setupFanout). When prefix is non-nil, CallLLM routes claude-*
	// section generation through the shared-prefix cache fan-out instead of
	// shelling `grove llm request`. forceModel, when set, overrides every
	// section's model for the run (so the whole wave shares one prefix).
	prefix         *anthropic.SharedPrefix
	forceModel     string
	currentSection string // label for per-section usage logging

	// usageRecords accumulates per-section fan-out usage over a run so it can be
	// emitted as a machine-readable report (GenerateOptions.UsageJSONPath).
	usageRecords []SectionUsage

	// failedSections accumulates the names of sections that failed during the
	// run (bare names in in-place mode, subdir/name in sections mode — the same
	// forms `-s` accepts) so the usage report can surface them and a shelling
	// caller can retry only the failed sections. failedSectionErrors carries
	// each failed section's error text so a shelling caller (grove release gen)
	// can classify the failure — transient vs permanent — across the exec
	// boundary instead of seeing only "exit status 1".
	failedSections      []string
	failedSectionErrors map[string]string
}

// GenerateOptions configures what sections to generate
type GenerateOptions struct {
	Sections []string // List of section names to generate (empty means all)
	Model    string   // Override the model for all sections (enables claude-* fan-out)
	CacheTTL string   // Fan-out shared-prefix cache TTL: "5m" (default) or "1h"
	// UsageJSONPath, when non-empty, is a file path to which a machine-readable
	// per-section cache/usage report (UsageReport) is written at the end of the
	// run. Callers that shell `docgen generate` (e.g. `grove release gen`) parse
	// this instead of scraping the human-readable log lines. Only fan-out
	// (claude-*) sections contribute records; a non-fan-out run writes an empty
	// report so the caller can still distinguish "ran, no cache usage" from
	// "did not run".
	UsageJSONPath string
}

// SectionUsage is one section's cache/usage accounting in the machine-readable
// usage report written by `docgen generate --usage-json`.
type SectionUsage struct {
	Section          string  `json:"section"`
	Model            string  `json:"model"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	EstCostUSD       float64 `json:"est_cost_usd"`
}

// UsageReport is the machine-readable per-run usage summary emitted by
// `docgen generate --usage-json <file>`. Totals are the sum over Sections.
// FailedSections lists the sections that failed this run, in a form `-s`
// accepts, so a caller can retry only those; it is always present (empty on a
// clean run) so callers can distinguish "no failures" from a report written by
// an older docgen that does not know the field.
type UsageReport struct {
	Model          string         `json:"model"`
	Sections       []SectionUsage `json:"sections"`
	FailedSections []string       `json:"failed_sections"`
	// FailedSectionErrors maps each failed section to its error text so a
	// shelling caller can show the real cause and classify it (e.g. an API 400
	// "prompt is too long" is permanent and must not be retried). Absent from
	// reports written by older docgen binaries.
	FailedSectionErrors   map[string]string `json:"failed_section_errors,omitempty"`
	TotalInputTokens      int64             `json:"total_input_tokens"`
	TotalOutputTokens     int64             `json:"total_output_tokens"`
	TotalCacheWriteTokens int64             `json:"total_cache_write_tokens"`
	TotalCacheReadTokens  int64             `json:"total_cache_read_tokens"`
	TotalEstCostUSD       float64           `json:"total_est_cost_usd"`
}

func New(logger *logrus.Logger) *Generator {
	return &Generator{logger: logger}
}

// recordSectionFailure books one failed section for the usage report and
// emits it with the section name in the log message itself — log panes list
// only the message line, and fifteen bare "Section failed" rows are useless
// without a click-through — plus the error text as a field.
func (g *Generator) recordSectionFailure(name string, err error) {
	g.failedSections = append(g.failedSections, name)
	if g.failedSectionErrors == nil {
		g.failedSectionErrors = make(map[string]string)
	}
	g.failedSectionErrors[name] = err.Error()
	ulog.Error(fmt.Sprintf("Section %q failed", name)).
		Field("section", name).
		Field("error", err.Error()).
		Emit()
}

// failedSectionsError builds the run-level error for a set of failed
// sections. Section failures share one root cause almost always (an
// over-window prefix 400s every request), so the first failure's error text
// rides along — without it the run error names the casualties but not the
// cause, and a shelling caller sees only "exit status 1".
func (g *Generator) failedSectionsError(failed []string) error {
	first := g.failedSectionErrors[failed[0]]
	if first == "" {
		return fmt.Errorf("%d section(s) failed: %s", len(failed), strings.Join(failed, ", "))
	}
	return fmt.Errorf("%d section(s) failed: %s; first error: %s", len(failed), strings.Join(failed, ", "), first)
}

// Generate orchestrates an isolated documentation generation process for all sections.
func (g *Generator) Generate(packageDir string) error {
	return g.GenerateWithOptions(packageDir, GenerateOptions{})
}

// GenerateWithOptions orchestrates documentation generation with specific options.
func (g *Generator) GenerateWithOptions(packageDir string, opts GenerateOptions) error {
	// Emit the machine-readable usage report at the end of the run (even on
	// partial failure) so a shelling caller always gets whatever was billed.
	if opts.UsageJSONPath != "" {
		defer g.writeUsageReport(opts.UsageJSONPath, opts.Model)
	}
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
// Resolution is delegated to resolvePromptPath so the pre-spend prompt guard and
// the actual generation read the SAME file (they can never diverge).
func (g *Generator) resolvePromptContent(packageDir, promptFile string) ([]byte, error) {
	path, err := g.resolvePromptPath(packageDir, promptFile)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// resolvePromptPath locates a prompt file WITHOUT reading it, following the
// exact resolution order generation uses:
// 1. Tries to resolve the workspace and get the notebook prompts directory
// 2. Looks for the prompt in the notebook directory (using basename only)
// 3. Falls back to the legacy path in docs/
// Returns the path of the file that exists, or an error naming every location
// tried. This is the single source of truth for prompt resolution — the
// pre-spend guard (validateSectionPrompts) and resolvePromptContent both use it.
func (g *Generator) resolvePromptPath(packageDir, promptFile string) (string, error) {
	// Extract basename only - ignore any directory prefix for backward compatibility
	promptBaseName := filepath.Base(promptFile)
	legacyPath := filepath.Join(packageDir, "docs", promptFile)

	// 1. Try to get workspace node for the package directory
	node, err := workspace.GetProjectByPath(packageDir)
	if err != nil {
		// Fallback: Can't resolve workspace, use legacy path
		g.logger.Warnf("Could not resolve workspace for %s. Falling back to legacy prompt path.", packageDir)
		if _, serr := os.Stat(legacyPath); serr != nil {
			return "", fmt.Errorf("prompt '%s' not found at legacy location (%s): %w", promptBaseName, legacyPath, serr)
		}
		return legacyPath, nil
	}

	// 2. Try notebook path first
	cfg, err := coreConfig.LoadDefault()
	if err == nil {
		locator := workspace.NewNotebookLocator(cfg)
		notebookPromptsDir, err := locator.GetDocgenPromptsDir(node)

		if err == nil {
			notebookPath := filepath.Join(notebookPromptsDir, promptBaseName)
			if _, serr := os.Stat(notebookPath); serr == nil {
				g.logger.Debugf("Resolved prompt '%s' from notebook: %s", promptBaseName, notebookPath)
				return notebookPath, nil
			}
		}
	}

	// 3. Fallback to legacy path
	g.logger.Debugf("Prompt not found in notebook, trying legacy path: %s", legacyPath)
	if _, serr := os.Stat(legacyPath); serr != nil {
		// Enhanced error message showing both paths attempted
		notebookPromptsDir := "unable to resolve"
		if cfg, cfgErr := coreConfig.LoadDefault(); cfgErr == nil {
			locator := workspace.NewNotebookLocator(cfg)
			if dir, dirErr := locator.GetDocgenPromptsDir(node); dirErr == nil {
				notebookPromptsDir = dir
			}
		}
		return "", fmt.Errorf(
			"prompt '%s' not found in notebook (%s) or legacy location (%s)",
			promptBaseName, notebookPromptsDir, legacyPath,
		)
	}

	return legacyPath, nil
}

// generateInPlace runs the core doc generation logic within a given directory.
func (g *Generator) generateInPlace(packageDir string, opts GenerateOptions) error {
	g.logger.Infof("Generating documentation in: %s", packageDir)

	// 1. Load config from the package directory (tries notebook first, then repo)
	cfg, configPath, err := config.LoadWithNotebook(packageDir)
	if err != nil {
		return fmt.Errorf("failed to load docgen config: %w", err)
	}

	// Resolve once, before building context or making any LLM request. A
	// configured docgen run must never silently fall back to default rules.
	rulesPath, err := config.ResolveDocsRulesFile(packageDir)
	if err != nil {
		return fmt.Errorf("failed to resolve docs rules: %w", err)
	}

	// Handle "sections" output mode: delegate to subdirectory-based generation
	if cfg.Settings.OutputMode == "sections" {
		return g.generateSectionsMode(packageDir, configPath, cfg, rulesPath, opts)
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

	// 3. Build context using the explicitly resolved rules artifact.
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.BuildContext(packageDir, rulesPath); err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	// 3a. Enable Claude cache fan-out for this run when applicable. Must run
	// after BuildContext so the cx context exists to form the shared prefix.
	// An over-window context is a hard, permanent error — see setupFanout.
	teardownFanout, err := g.setupFanout(packageDir, cfg, opts)
	if err != nil {
		return err
	}
	defer teardownFanout()

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

		// Filter sections and validate. Config sections may legitimately share
		// a name (e.g. a production and a draft "overview" with different
		// outputs), so a requested name selects EVERY section bearing it —
		// deleting on first match would silently drop the duplicates from a
		// retry of failed sections.
		var filteredSections []config.SectionConfig
		var invalidSections []string
		foundSections := make(map[string]bool)

		for _, section := range cfg.Sections {
			// Check if this section was requested
			if requestedSections[section.Name] {
				filteredSections = append(filteredSections, section)
				foundSections[section.Name] = true
			}
		}

		// Check for any requested sections that weren't found
		for name := range requestedSections {
			if !foundSections[name] {
				invalidSections = append(invalidSections, name)
			}
		}

		if len(invalidSections) > 0 {
			return fmt.Errorf("sections not found in config: %v", invalidSections)
		}

		sectionsToGenerate = filteredSections
		g.logger.Infof("Generating %d of %d sections: %v", len(sectionsToGenerate), len(cfg.Sections), opts.Sections)
	}

	// Pre-spend guard: fail before any LLM call if an in-scope section lacks an
	// output: filename (an empty output writes onto the output dir itself). Only
	// the sections this run will actually generate are validated.
	if err := validateSectionOutputs(sectionsToGenerate); err != nil {
		return err
	}

	// Pre-spend guard: every in-scope prose section's prompt file must resolve
	// (notebook first, legacy fallback — the same resolution the loop below
	// uses) before any LLM call, listing ALL missing prompts in one error.
	if err := validateSectionPrompts(sectionsToGenerate, func(_ int, s config.SectionConfig) (string, error) {
		return g.resolvePromptPath(packageDir, s.Prompt)
	}); err != nil {
		return err
	}

	// 5. Generate each section. Failures don't abort the run (later sections
	// still get their chance to generate), but they must not vanish either:
	// callers like `grove release gen` rely on the exit code to decide whether
	// a repo's docs are actually staged, so every failed section is surfaced
	// and the run as a whole errors at the end.
	var failedSections []string
	sectionFailed := func(name string, err error) {
		failedSections = append(failedSections, name)
		g.recordSectionFailure(name, err)
	}
	for _, section := range sectionsToGenerate {
		g.currentSection = section.Name
		// Handle different generation types
		if section.Type == "schema_to_md" {
			if err := g.generateFromSchema(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Schema to Markdown generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "schema_table" {
			if err := g.generateFromSchemaTable(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Schema table generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "schema_describe" {
			if err := g.generateSchemaDescriptions(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Schema descriptions generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "schema_examples" {
			if err := g.generateSchemaExamples(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Schema examples generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "doc_sections" {
			if err := g.generateFromDocSections(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Doc sections generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "capture" {
			if err := g.generateFromCapture(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("CLI capture generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "nb_concept" {
			if err := g.generateFromConcept(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("Concept generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "tui_keymaps" {
			if err := g.generateFromTUIKeymaps(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("TUI keymaps generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
			}
			continue
		}
		if section.Type == "tui_describe" {
			if err := g.generateTUIDescriptions(packageDir, section, cfg, outputBaseDir); err != nil {
				g.logger.WithError(err).Errorf("TUI descriptions generation failed for section '%s'", section.Name)
				sectionFailed(section.Name, err)
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
			sectionFailed(section.Name, err)
			continue // Continue to the next section even if one fails
		}

		// 6. Write output to the determined output directory
		outputPath := filepath.Join(outputBaseDir, section.Output)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil { //nolint:gosec // internal doc tool
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
			return fmt.Errorf("failed to write section output: %w", err)
		}
		g.logger.Infof("Successfully wrote section '%s' to %s", section.Name, outputPath)
		ulog.Success("Wrote section").
			Field("section", section.Name).
			Field("path", outputPath).
			Emit()
	}

	if len(failedSections) > 0 {
		return g.failedSectionsError(failedSections)
	}
	return nil
}

// validateSectionOutputs is the pre-spend guard for a generation run: every
// section about to be generated MUST carry an output: filename, or the write
// after a paid-for LLM call fails with "open <dir>: is a directory" (an empty
// output joins onto the output dir and resolves to the dir itself); a capture
// section MUST carry binary:, or its dispatch fails mid-run after earlier
// sections already paid for their LLM calls. It errors BEFORE any LLM call,
// naming every offending in-scope section in one message. Two fragments are
// load-bearing for grove's release retry-classifier (they mark the failure
// permanent, no retry): "has no output: filename" and the
// "section type 'capture' requires 'binary'" config-validation family.
func validateSectionOutputs(sections []config.SectionConfig) error {
	var missing []string
	var captures []string
	for _, s := range sections {
		if strings.TrimSpace(s.Output) == "" {
			missing = append(missing, s.Name)
		}
		if s.Type == "capture" && strings.TrimSpace(s.Binary) == "" {
			captures = append(captures, s.Name)
		}
	}
	var parts []string
	switch len(missing) {
	case 0:
	case 1:
		parts = append(parts, fmt.Sprintf("section %q has no output: filename", missing[0]))
	default:
		parts = append(parts, fmt.Sprintf("section %q has no output: filename (%d more: %s)",
			missing[0], len(missing)-1, strings.Join(missing[1:], ", ")))
	}
	for _, name := range captures {
		parts = append(parts, fmt.Sprintf("section %q: section type 'capture' requires 'binary' (binary name)", name))
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("docs config error: %s", strings.Join(parts, "; "))
}

// validateSectionPrompts is the pre-spend prompt-existence guard, the prompt
// counterpart to validateSectionOutputs: every in-scope PROSE section's prompt
// file must resolve BEFORE any LLM call, or a section late in the run would
// hard-fail on a missing prompt after earlier sections already paid for their
// calls. resolvePath is the caller's own prompt resolution (index-aligned with
// sections) so the guard checks the exact file generation would read — notebook
// first + legacy fallback in generateInPlace, the subdir prompts/ dir in
// generateSectionsMode. All missing prompts are listed in ONE error. The
// "could not resolve prompt" fragment is load-bearing for grove's release
// retry classifier (it marks the failure permanent — no retry re-spend).
func validateSectionPrompts(sections []config.SectionConfig, resolvePath func(i int, s config.SectionConfig) (string, error)) error {
	var problems []string
	for i, s := range sections {
		if !isProseSection(s.Type) {
			continue
		}
		if strings.TrimSpace(s.Prompt) == "" {
			problems = append(problems, fmt.Sprintf("section %q has no prompt: filename", s.Name))
			continue
		}
		if _, err := resolvePath(i, s); err != nil {
			problems = append(problems, fmt.Sprintf("section %q: %v", s.Name, err))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("could not resolve prompt for %d prose section(s) — failing before any LLM call: %s",
		len(problems), strings.Join(problems, "; "))
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
- If "Important: true" → prefix the property name with ★ in the table

Example description with badges:
"(object, optional) Settings for embedded Neovim. <span class="schema-badge schema-badge-alpha">ALPHA</span> <span class="schema-status-msg">Experimental feature</span>"
---
`

const SchemaExamplesSystemPromptTOML = `You are generating example TOML configurations for the documented tool.

**Input Context:**
- Source code/documentation to understand usage patterns.
- List of configuration properties with types.

**Output Format:**
Return a JSON object where keys are property paths and values are objects containing an 'example' (TOML snippet) and a 'description'.

JSON Output Structure:
{
  "property.path": {
    "example": "key = \"value\"",
    "description": "Brief explanation of the example."
  }
}

**Guidelines:**
- Use realistic values (not "foo", "bar").
- Use proper TOML syntax (handle nested tables [table] vs inline dictionaries {}).
- Provide comments in the TOML snippet if helpful.
- For lists, show multiple items.
- Focus on common use cases.
`

const SchemaExamplesSystemPromptYAML = `You are generating example YAML configurations for markdown frontmatter.

**Input Context:**
- Source code/documentation to understand usage patterns.
- List of configuration properties with types.

**Output Format:**
Return a JSON object where keys are property paths and values are objects containing an 'example' (YAML frontmatter snippet) and a 'description'.

JSON Output Structure:
{
  "property.path": {
    "example": "key: value",
    "description": "Brief explanation of the example."
  }
}

**Guidelines:**
- Use realistic values (not "foo", "bar").
- Use proper YAML syntax for frontmatter (key: value format).
- For lists, use YAML array syntax (- item or [item1, item2]).
- For nested objects, use proper indentation.
- Do NOT include the --- fences, just the YAML content.
- Focus on common use cases.
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
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil { //nolint:gosec // internal doc tool
		return fmt.Errorf("failed to create output directory for schema doc: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
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
		var docInfo *SchemaDocInfo
		if source.Doc == "" {
			docInfo = g.findSchemaDocInfo(pkgPath)
			if docInfo == nil {
				return fmt.Errorf("could not auto-discover schema doc for package '%s' (no schema_to_md or schema_table section found)", source.Package)
			}
			g.logger.Debugf("Auto-discovered schema doc for %s: %s (type: %s)", source.Package, docInfo.MarkdownPath, docInfo.Type)
		} else {
			// Explicit doc path specified
			docInfo = &SchemaDocInfo{
				MarkdownPath: source.Doc,
				Type:         "explicit",
			}
		}

		// Add section info
		sb.WriteString("\n--- SECTION ---\n")
		if source.Title != "" {
			sb.WriteString(fmt.Sprintf("Title: %s\n", source.Title))
		} else {
			sb.WriteString(fmt.Sprintf("Title: %s Configuration\n", source.Package))
		}
		if source.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", source.Description))
		}
		sb.WriteString(fmt.Sprintf("Properties to include: %v\n", source.Properties))
		sb.WriteString(fmt.Sprintf("Package: %s\n\n", source.Package))

		// For schema_table with JSON, read the structured data files
		if docInfo.Type == "schema_table" && docInfo.JSONPath != "" {
			// Read the configuration JSON (has full property tree with descriptions)
			if jsonPath := g.resolveDocPath(pkgPath, docInfo.JSONPath); jsonPath != "" {
				if jsonContent, err := os.ReadFile(jsonPath); err == nil {
					sb.WriteString("--- CONFIGURATION DATA (JSON) ---\n")
					sb.WriteString(string(jsonContent))
					sb.WriteString("\n")
				}
			}

			// Read examples JSON if available
			if docInfo.ExamplesPath != "" {
				if examplesPath := g.resolveDocPath(pkgPath, docInfo.ExamplesPath); examplesPath != "" {
					if examplesContent, err := os.ReadFile(examplesPath); err == nil {
						sb.WriteString("--- EXAMPLES (JSON) ---\n")
						sb.WriteString(string(examplesContent))
						sb.WriteString("\n")
					}
				}
			}
		} else {
			// Fall back to reading markdown content
			docPath := g.resolveDocPath(pkgPath, docInfo.MarkdownPath)
			if docPath == "" {
				return fmt.Errorf("doc file '%s' not found for package '%s'", docInfo.MarkdownPath, source.Package)
			}

			content, err := os.ReadFile(docPath)
			if err != nil {
				return fmt.Errorf("failed to read doc %s: %w", docPath, err)
			}
			sb.WriteString("--- SOURCE DOCUMENTATION ---\n")
			sb.WriteString(string(content))
		}
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
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil { //nolint:gosec // internal doc tool
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("failed to write doc sections output: %w", err)
	}
	g.logger.Infof("Successfully wrote doc sections '%s' to %s", section.Name, outputPath)
	return nil
}

// SchemaDocInfo contains paths to schema documentation files
type SchemaDocInfo struct {
	// MarkdownPath is the path to the markdown file (for schema_to_md or schema_table)
	MarkdownPath string
	// JSONPath is the path to the JSON data file (for schema_table with format: json)
	JSONPath string
	// DescriptionsPath is the path to the descriptions JSON file
	DescriptionsPath string
	// ExamplesPath is the path to the examples JSON file
	ExamplesPath string
	// Type is "schema_to_md" or "schema_table"
	Type string
}

// findSchemaDocInfo looks for schema documentation sections and returns detailed info.
func (g *Generator) findSchemaDocInfo(pkgPath string) *SchemaDocInfo {
	// Try to load the package's docgen config
	cfg, _, err := config.LoadWithNotebook(pkgPath)
	if err != nil {
		return nil
	}

	// Look for schema_table first (preferred), then schema_to_md
	var schemaTableSection *config.SectionConfig
	var schemaToMdSection *config.SectionConfig

	for i := range cfg.Sections {
		section := &cfg.Sections[i]
		if section.Type == "schema_table" && schemaTableSection == nil {
			schemaTableSection = section
		}
		if section.Type == "schema_to_md" && schemaToMdSection == nil {
			schemaToMdSection = section
		}
	}

	// Prefer schema_table
	if schemaTableSection != nil {
		info := &SchemaDocInfo{
			MarkdownPath: "docs/" + schemaTableSection.Output,
			Type:         "schema_table",
		}
		// If format is json, there's a companion JSON file
		if schemaTableSection.Format == "json" {
			jsonFile := strings.TrimSuffix(schemaTableSection.Output, ".md") + ".json"
			info.JSONPath = "docs/" + jsonFile
		}
		// Check for descriptions and examples references
		if schemaTableSection.Descriptions != "" {
			info.DescriptionsPath = "docs/" + schemaTableSection.Descriptions
		}
		if schemaTableSection.Examples != "" {
			info.ExamplesPath = "docs/" + schemaTableSection.Examples
		}
		return info
	}

	// Fall back to schema_to_md
	if schemaToMdSection != nil {
		return &SchemaDocInfo{
			MarkdownPath: "docs/" + schemaToMdSection.Output,
			Type:         "schema_to_md",
		}
	}

	return nil
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
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write schema table output: %w", err)
	}

	g.logger.Infof("Successfully wrote schema table '%s' to %s", section.Name, outputPath)
	return nil
}

// ConfigNode represents a configuration property in the JSON output for the website.
// It preserves the hierarchical structure and includes all metadata for rich UI rendering.
type ConfigNode struct {
	Name             string       `json:"name"`
	Path             string       `json:"path"` // Full dotted path (e.g., "groves.mygrove.enabled")
	Type             string       `json:"type"`
	Description      string       `json:"description"`
	Required         bool         `json:"required,omitempty"`
	Default          interface{}  `json:"default,omitempty"`
	Deprecated       bool         `json:"deprecated,omitempty"`
	Layer            string       `json:"layer,omitempty"` // global, ecosystem, project
	Priority         int          `json:"priority,omitempty"`
	Important        bool         `json:"important,omitempty"` // Key configuration field (★)
	Hint             string       `json:"hint,omitempty"`
	Status           string       `json:"status,omitempty"` // alpha, beta, stable, deprecated
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
	if err := os.MkdirAll(outputBaseDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write JSON data file
	jsonPath := filepath.Join(outputBaseDir, jsonOutput)
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config schema to JSON: %w", err)
	}

	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write schema table JSON output: %w", err)
	}
	g.logger.Infof("Successfully wrote schema table JSON '%s' to %s", section.Name, jsonPath)

	// Write markdown wrapper file with config-reference code block
	if mdOutput != "" {
		// The JSON will be served from /data/{package}/{filename}.json
		// Use the package directory name (workspace name) as the package identifier
		packageName := filepath.Base(packageDir)

		// Build the config-reference JSON
		configRefJSON := fmt.Sprintf(`{"src": "/data/%s/%s"`, packageName, jsonOutput)
		if section.Examples != "" {
			// Extract just the filename from the examples path
			examplesFile := filepath.Base(section.Examples)
			configRefJSON += fmt.Sprintf(`, "examplesSrc": "/data/%s/%s"`, packageName, examplesFile)
			// Include format if specified (default is toml)
			if section.ExamplesFormat != "" {
				configRefJSON += fmt.Sprintf(`, "examplesFormat": "%s"`, section.ExamplesFormat)
			}
		}
		configRefJSON += "}"

		mdContent := fmt.Sprintf(`# %s

`+"```config-reference"+`
%s
`+"```"+`
`, section.Title, configRefJSON)

		mdPath := filepath.Join(outputBaseDir, mdOutput)
		if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
			return fmt.Errorf("failed to write schema table markdown wrapper: %w", err)
		}
		g.logger.Infof("Successfully wrote schema table markdown wrapper to %s", mdPath)
	}

	return nil
}

// schemaPropsToConfigNodes converts schema.Property slice to ConfigNode slice
func (g *Generator) schemaPropsToConfigNodes(props []schema.Property, prefix string, descriptions map[string]string) []ConfigNode {
	nodes := make([]ConfigNode, 0, len(props))

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
			Important:        prop.Important,
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
	if prop.Important {
		name = "★ " + name
	}
	if prop.Deprecated {
		name = "~~" + name + "~~"
	}

	// Layer column with badge-style formatting
	layer := ""
	if prop.Layer != "" {
		layer = fmt.Sprintf("**%s**", cases.Title(language.English).String(prop.Layer))
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

	if section.RulesFile != "" {
		if err := g.BuildContextForRulesSpec(packageDir, section.RulesFile); err != nil {
			return fmt.Errorf("failed to build section context: %w", err)
		}
	}

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
		model = "gemini-3-pro-preview"
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
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(descriptions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal descriptions: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write descriptions file: %w", err)
	}

	g.logger.Infof("Successfully wrote %d descriptions to %s", len(descriptions), outputPath)
	return nil
}

// generateSchemaExamples generates realistic TOML/YAML examples for schema properties.
func (g *Generator) generateSchemaExamples(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating schema examples: %s", section.Name)

	// Normalize inputs
	var inputs []config.SchemaInput
	if len(section.Schemas) > 0 {
		inputs = section.Schemas
	} else if section.Source != "" {
		inputs = []config.SchemaInput{{Path: section.Source}}
	} else {
		return fmt.Errorf("section type 'schema_examples' requires 'schemas' list or 'source' file")
	}

	// Collect all properties
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

	// Build Prompt
	var promptBuilder strings.Builder

	if section.RulesFile != "" {
		if err := g.BuildContextForRulesSpec(packageDir, section.RulesFile); err != nil {
			return fmt.Errorf("failed to build section context: %w", err)
		}
	}

	// Select prompt based on format (default: toml)
	exampleFormat := section.Format
	if exampleFormat == "" {
		exampleFormat = "toml"
	}
	if exampleFormat == "yaml" {
		promptBuilder.WriteString(SchemaExamplesSystemPromptYAML)
	} else {
		promptBuilder.WriteString(SchemaExamplesSystemPromptTOML)
	}

	// Add TOML section header instruction if specified
	if section.TomlSection != "" && exampleFormat != "yaml" {
		promptBuilder.WriteString(fmt.Sprintf("\n\n**IMPORTANT:** All TOML examples MUST start with the section header `[%s]` on its own line, followed by the property. For example:\n```\n[%s]\nproperty = value\n```\n", section.TomlSection, section.TomlSection))
	}

	promptBuilder.WriteString("\n\n## Properties to Generate Examples For\n")
	g.collectPropertyPaths(&promptBuilder, allProps, "")

	promptBuilder.WriteString(`
Output format (JSON only, no markdown fences):
{
  "property.path": {
    "example": "...",
    "description": "..."
  }
}
`)

	// Call LLM
	model := section.Model
	if model == "" {
		model = cfg.Settings.Model
	}
	if model == "" {
		model = "gemini-3-pro-preview"
	}

	genConfig := config.MergeGenerationConfig(cfg.Settings.GenerationConfig, section.GenerationConfig)
	response, err := g.CallLLM(promptBuilder.String(), model, genConfig, packageDir)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse Response (Expects JSON map[string]struct{Example, Description})
	type ExampleEntry struct {
		Example     string `json:"example"`
		Description string `json:"description"`
	}
	var examples map[string]ExampleEntry

	cleanResponse := strings.TrimSpace(response)
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	if err := json.Unmarshal([]byte(cleanResponse), &examples); err != nil {
		return fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	// Write Output
	outputPath := filepath.Join(outputBaseDir, section.Output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(examples, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal examples: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write examples file: %w", err)
	}

	g.logger.Infof("Successfully wrote %d examples to %s", len(examples), outputPath)
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
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory for capture: %w", err)
	}

	if err := capturer.Capture(section.Binary, outputPath, opts); err != nil {
		return fmt.Errorf("CLI capture failed for section '%s': %w", section.Name, err)
	}

	g.logger.Infof("Successfully captured CLI reference for '%s' to %s", section.Binary, outputPath)
	return nil
}

// BuildContext runs cx generate to prepare context for LLM calls
func (g *Generator) BuildContext(packageDir, rulesPath string) error {
	args := []string{"generate"}
	if rulesPath != "" {
		g.logger.Infof("Docs context rules: %s", rulesPath)
		ulog.Info("Docs context rules active").Field("rules", rulesPath).Emit()
		args = append(args, "--rules-file", rulesPath)
	}
	cmd := delegation.Command("cx", args...)
	cmd.Dir = packageDir
	// Discard output to avoid contaminating the LLM response
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// BuildContextForRulesSpec regenerates context for a section-specific rules
// selection. Unlike the removed .grove/rules staging, the LLM sees exactly the
// artifact selected by the section's explicit rules_file.
func (g *Generator) BuildContextForRulesSpec(packageDir, spec string) error {
	rulesPath, err := config.ResolveRulesFileSpec(packageDir, spec)
	if err != nil {
		return err
	}
	return g.BuildContext(packageDir, rulesPath)
}

// CallLLM makes an LLM request with the given prompt and configuration.
//
// When a cache fan-out prefix is active for this run (setupFanout) and the
// effective model is the prefix's Claude model, the request is issued as a
// task rider on the shared, byte-identical repo-context prefix — the big cx
// context is written to the cache once and cache-read by every subsequent
// section — instead of shelling `grove llm request`. Non-Claude models (and
// runs without an active prefix) keep the original facade path untouched.
func (g *Generator) CallLLM(promptContent, model string, genConfig config.GenerationConfig, workDir string) (string, error) {
	// A run-wide --model override forces every section onto one model so the
	// whole wave shares a single cached prefix.
	if g.forceModel != "" {
		model = g.forceModel
	}

	// Use provided model or default to gemini-3-pro-preview
	if model == "" {
		model = "gemini-3-pro-preview"
	}

	// Route Claude generation through the shared-prefix fan-out when one is
	// active for this exact model.
	if g.prefix != nil && anthropic.ResolveModelAlias(model) == g.prefix.Model() {
		return g.callViaFanout(promptContent)
	}

	// Create a temporary file for the prompt
	promptFile, err := os.CreateTemp("", "docgen-prompt-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp prompt file: %w", err)
	}
	defer os.Remove(promptFile.Name()) //nolint:errcheck // best-effort temp cleanup

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
		"--regenerate", // Ensure context is regenerated with current rules
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
		// Log the full stderr for debugging
		g.logger.Debugf("LLM stderr: %s", stderr.String())
		// Surface the underlying cause (e.g. missing API key) instead of
		// just "exit status 1" by including the tail of stderr.
		if tail := lastLines(stderr.String(), 10); tail != "" {
			return "", fmt.Errorf("grove llm request failed: %w; stderr:\n%s", err, tail)
		}
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
	return cleanLLMResponse(string(output)), nil
}

// cleanLLMResponse trims whitespace and strips a single wrapping markdown code
// fence (```markdown / ```md / ```) from an LLM response, leaving clean markdown.
// Shared by the shell facade path and the cache fan-out path so both produce
// byte-comparable section output.
func cleanLLMResponse(response string) string {
	response = strings.TrimSpace(response)

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

	return response
}

// callViaFanout issues one section request against the active shared-prefix
// cache fan-out and logs its per-section cache write/read usage.
func (g *Generator) callViaFanout(promptContent string) (string, error) {
	text, usage, err := g.prefix.Request(context.Background(), promptContent)
	g.logFanoutUsage(usage)
	if err != nil {
		return "", fmt.Errorf("cache fan-out request failed: %w", err)
	}
	return cleanLLMResponse(text), nil
}

// logFanoutUsage prints the per-section cache write/read token accounting so
// caching is verifiable from the CLI: request 1 shows a large cache_write and
// near-zero cache_read; every subsequent section shows cache_read ≈ prefix size.
func (g *Generator) logFanoutUsage(u *anthropic.UsageResult) {
	if u == nil {
		return
	}
	section := g.currentSection
	if section == "" {
		section = "(section)"
	}
	g.logger.Infof("Cache usage [%s]: model=%s input=%d output=%d cache_write=%d (5m=%d 1h=%d) cache_read=%d est_cost=$%.4f",
		section, u.Model, u.InputTokens, u.OutputTokens, u.CacheCreationTokens, u.CacheWrite5m, u.CacheWrite1h, u.CacheReadTokens, u.EstimatedCostUSD)
	g.usageRecords = append(g.usageRecords, SectionUsage{
		Section:          section,
		Model:            u.Model,
		InputTokens:      u.InputTokens,
		OutputTokens:     u.OutputTokens,
		CacheWriteTokens: u.CacheCreationTokens,
		CacheReadTokens:  u.CacheReadTokens,
		EstCostUSD:       u.EstimatedCostUSD,
	})
	ulog.Info("Cache fan-out usage").
		Field("section", section).
		Field("model", u.Model).
		Field("input", u.InputTokens).
		Field("output", u.OutputTokens).
		Field("cache_write", u.CacheCreationTokens).
		Field("cache_read", u.CacheReadTokens).
		Field("est_cost_usd", u.EstimatedCostUSD).
		Emit()
}

// writeUsageReport serializes the run's accumulated per-section fan-out usage to
// path as a UsageReport JSON document. It is best-effort: a write failure is
// logged but never fails the generation run. reqModel is the run's requested
// model override (may be empty); the report's Model prefers the model actually
// billed (from the first record) and falls back to reqModel.
func (g *Generator) writeUsageReport(path, reqModel string) {
	report := UsageReport{Model: reqModel, Sections: g.usageRecords, FailedSections: g.failedSections, FailedSectionErrors: g.failedSectionErrors}
	if report.Sections == nil {
		report.Sections = []SectionUsage{}
	}
	if report.FailedSections == nil {
		report.FailedSections = []string{}
	}
	for _, r := range g.usageRecords {
		if report.Model == "" || report.Model == reqModel {
			if r.Model != "" {
				report.Model = r.Model
			}
		}
		report.TotalInputTokens += r.InputTokens
		report.TotalOutputTokens += r.OutputTokens
		report.TotalCacheWriteTokens += r.CacheWriteTokens
		report.TotalCacheReadTokens += r.CacheReadTokens
		report.TotalEstCostUSD += r.EstCostUSD
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		g.logger.WithError(err).Warnf("failed to marshal usage report for %s", path)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		g.logger.WithError(err).Warnf("failed to write usage report to %s", path)
		return
	}
	g.logger.Infof("Wrote usage report: %s (%d sections, cache_write=%d cache_read=%d est_cost=$%.4f)",
		path, len(report.Sections), report.TotalCacheWriteTokens, report.TotalCacheReadTokens, report.TotalEstCostUSD)
}

// docsWindowTokens is the context-window budget the docs fan-out prefix must
// fit inside. All current claude-* docs models have (at least) a 200k window;
// a prefix that exceeds it makes EVERY section request 400 with "prompt is
// too long", so the run fails up front instead.
const docsWindowTokens = 200_000

// docsBytesPerToken is the conservative bytes-per-token estimate used for the
// window precheck. Real Go source runs ~3.1 bytes/token, so dividing by 3
// slightly over-counts tokens — erring toward failing fast rather than
// spending on a wave of guaranteed 400s.
const docsBytesPerToken = 3

// setupFanout configures the run's cache fan-out when the effective model is a
// Claude model. It builds a shared prefix from the cx context that BuildContext
// has already generated for packageDir and returns a teardown func the caller
// must defer. When fan-out does not apply (non-Claude model, or no cx context),
// it is a no-op and CallLLM keeps shelling `grove llm request`.
//
// A context that cannot fit the model window is a hard error (not a fallback):
// the standard path would upload the same over-window context and fail every
// section with an API 400, so the run stops before any spend with a pointer at
// the configured rules_file preset.
func (g *Generator) setupFanout(packageDir string, cfg *config.DocgenConfig, opts GenerateOptions) (func(), error) {
	noop := func() {}

	prefixModel := opts.Model
	if prefixModel == "" {
		prefixModel = cfg.Settings.Model
	}
	if !anthropic.IsAnthropicModel(prefixModel) {
		if cfg.Settings.CacheFanout {
			g.logger.Warnf("cache_fanout is set but effective model %q is not a Claude model; using the standard grove llm path", prefixModel)
		}
		return noop, nil
	}

	// Verify the context fileset before spending: an empty set means cx produced
	// nothing to cache, so fall back rather than fan out over an empty prefix.
	ctxFiles := anthropic.WorkDirContextFiles(packageDir)
	if len(ctxFiles) == 0 {
		g.logger.Warnf("cache fan-out requested for model %q but cx produced no context in %s; using the standard grove llm path", prefixModel, packageDir)
		return noop, nil
	}

	// Window precheck — fail fast and loud before any upload/spend.
	if err := checkDocsWindow(prefixModel, ctxFiles); err != nil {
		return noop, err
	}

	ttl := opts.CacheTTL
	if ttl == "" {
		ttl = cfg.Settings.CacheTTL
	}
	if ttl == "" {
		ttl = "5m"
	}

	prefix, err := newDocsSharedPrefix(ctxFiles, prefixModel, ttl)
	if err != nil {
		g.logger.WithError(err).Warnf("failed to set up cache fan-out for model %q; using the standard grove llm path", prefixModel)
		return noop, nil
	}

	g.prefix = prefix
	g.forceModel = prefixModel
	g.logger.Infof("Cache fan-out enabled: model=%s ttl=%s prefix_docs=%d", prefix.Model(), ttl, len(ctxFiles))
	ulog.Info("Cache fan-out enabled").
		Field("model", prefix.Model()).
		Field("ttl", ttl).
		Field("prefix_docs", len(ctxFiles)).
		Emit()

	return func() {
		_ = prefix.Close()
		g.prefix = nil
		g.forceModel = ""
		g.currentSection = ""
	}, nil
}

// checkDocsWindow enforces the pre-spend context-window precheck against the
// docs context fileset. It is the single guard shared by the docs fan-out setup
// (setupFanout) and the propose "turn 0" (Propose) so both refuse an over-window
// prefix identically, before any Files-API upload or API spend. An over-window
// context is a hard, permanent error (the same over-window bytes would 400 every
// request), so the caller stops rather than falling back.
func checkDocsWindow(prefixModel string, ctxFiles []string) error {
	var ctxBytes int64
	for _, f := range ctxFiles {
		if fi, statErr := os.Stat(f); statErr == nil {
			ctxBytes += fi.Size()
		}
	}
	if estTokens := ctxBytes / docsBytesPerToken; estTokens > docsWindowTokens {
		err := fmt.Errorf(
			"docs context too large for %s: %d file(s), %.2f MB (~%dk tokens at ~%d bytes/token) exceeds the ~%dk-token window — narrow the configured settings.rules_file preset",
			prefixModel, len(ctxFiles), float64(ctxBytes)/1e6, estTokens/1000, docsBytesPerToken, docsWindowTokens/1000)
		ulog.Error("Docs context exceeds model window").
			Field("model", prefixModel).
			Field("context_bytes", ctxBytes).
			Field("est_tokens", estTokens).
			Field("window_tokens", docsWindowTokens).
			Field("docs_rules_remedy", "settings.rules_file").
			Emit()
		return err
	}
	return nil
}

// newDocsSharedPrefix builds the shared cx-context prefix byte-identically for
// the docs fan-out and the propose turn: the SAME cx-generated fileset (from
// anthropic.WorkDirContextFiles), the SAME empty system prompt, and the SAME
// single breakpoint. That byte-identity is the whole point — a prefix warmed by
// `docgen propose` is cache-READ by a later `docgen generate` (and the changelog
// rider) against the same repo/model, so the review turn's cache write is not
// re-paid at generation time. Callers MUST have run BuildContext first so the cx
// context exists on disk. MaxTokens is a per-request generation cap and does not
// participate in the cached prefix, so it need not match across callers.
func newDocsSharedPrefix(ctxFiles []string, model, ttl string) (*anthropic.SharedPrefix, error) {
	return anthropic.NewSharedPrefixFromFiles("", ctxFiles, anthropic.SharedPrefixOptions{
		Model:     model,
		TTL:       ttl,
		MaxTokens: 8192,
		Caller:    "docgen",
	})
}

// lastLines returns the last n lines of s, with surrounding whitespace trimmed.
// It returns "" if s contains no non-whitespace content.
func lastLines(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// generateSectionsMode handles output_mode: sections where the top-level config
// is a website content aggregator. Sections live in subdirectories (e.g., overview/,
// concepts/), each with their own docgen.config.yml. This method discovers those
// subdirectory configs, merges their sections, and generates from each.
func (g *Generator) generateSectionsMode(packageDir, configPath string, topCfg *config.DocgenConfig, rulesPath string, opts GenerateOptions) error {
	docgenDir := filepath.Dir(configPath)
	g.logger.Infof("Sections mode: scanning subdirectories in %s", docgenDir)
	ulog.Info("Sections mode").
		Field("docgenDir", docgenDir).
		Emit()

	// Build context once for the whole package
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.BuildContext(packageDir, rulesPath); err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	// Enable Claude cache fan-out for this run when applicable (after
	// BuildContext so the shared cx-context prefix exists). An over-window
	// context is a hard, permanent error — see setupFanout.
	teardownFanout, err := g.setupFanout(packageDir, topCfg, opts)
	if err != nil {
		return err
	}
	defer teardownFanout()

	// Discover subdirectories with their own docgen.config.yml
	type subSection struct {
		subDir  string // subdirectory path (e.g., .../docgen/overview)
		subCfg  *config.DocgenConfig
		section config.SectionConfig
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

	// Pre-spend guard: fail before any LLM call if an in-scope section lacks an
	// output: filename (an empty output writes onto the output dir itself). Names
	// are qualified (subdir/section) so the error points at the exact section.
	scoped := make([]config.SectionConfig, 0, len(sectionsToGenerate))
	for _, ss := range sectionsToGenerate {
		s := ss.section
		s.Name = qualifiedName(ss)
		scoped = append(scoped, s)
	}
	if err := validateSectionOutputs(scoped); err != nil {
		return err
	}

	// Pre-spend guard: every in-scope prose section's prompt file must exist in
	// its subdirectory's prompts/ dir (the exact path the loop below reads)
	// before any LLM call, listing ALL missing prompts in one error.
	if err := validateSectionPrompts(scoped, func(i int, _ config.SectionConfig) (string, error) {
		p := filepath.Join(sectionsToGenerate[i].subDir, "prompts", sectionsToGenerate[i].section.Prompt)
		if _, serr := os.Stat(p); serr != nil {
			return "", fmt.Errorf("prompt not found at %s", p)
		}
		return p, nil
	}); err != nil {
		return err
	}

	// Generate each section using its subdirectory context. As in
	// generateInPlace, per-section failures don't abort the run but are
	// collected and surfaced as a run-level error so callers see a nonzero
	// exit instead of a silent no-op.
	var failedSections []string
	sectionFailed := func(name string, err error) {
		failedSections = append(failedSections, name)
		g.recordSectionFailure(name, err)
	}
	for _, ss := range sectionsToGenerate {
		g.currentSection = qualifiedName(ss)
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
				sectionFailed(qualifiedName(ss), err)
			}
			continue
		}
		if ss.section.Type == "schema_table" {
			if err := g.generateFromSchemaTable(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Schema table generation failed for section '%s'", ss.section.Name)
				sectionFailed(qualifiedName(ss), err)
			}
			continue
		}
		if ss.section.Type == "schema_describe" {
			if err := g.generateSchemaDescriptions(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Schema descriptions generation failed for section '%s'", ss.section.Name)
				sectionFailed(qualifiedName(ss), err)
			}
			continue
		}
		if ss.section.Type == "schema_examples" {
			if err := g.generateSchemaExamples(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Schema examples generation failed for section '%s'", ss.section.Name)
				sectionFailed(qualifiedName(ss), err)
			}
			continue
		}
		if ss.section.Type == "doc_sections" {
			if err := g.generateFromDocSections(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("Doc sections generation failed for section '%s'", ss.section.Name)
				sectionFailed(qualifiedName(ss), err)
			}
			continue
		}
		if ss.section.Type == "capture" {
			if err := g.generateFromCapture(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("CLI capture generation failed for section '%s'", ss.section.Name)
				sectionFailed(qualifiedName(ss), err)
			}
			continue
		}
		if ss.section.Type == "tui_keymaps" {
			if err := g.generateFromTUIKeymaps(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("TUI keymaps generation failed for section '%s'", ss.section.Name)
				sectionFailed(qualifiedName(ss), err)
			}
			continue
		}
		if ss.section.Type == "tui_describe" {
			if err := g.generateTUIDescriptions(packageDir, ss.section, ss.subCfg, outputDir); err != nil {
				g.logger.WithError(err).Errorf("TUI descriptions generation failed for section '%s'", ss.section.Name)
				sectionFailed(qualifiedName(ss), err)
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
			sectionFailed(qualifiedName(ss), err)
			continue
		}

		// Write output to the subdirectory's docs/ folder
		outputPath := filepath.Join(outputDir, ss.section.Output)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
			return fmt.Errorf("failed to write section output: %w", err)
		}
		g.logger.Infof("Successfully wrote section '%s' to %s", ss.section.Name, outputPath)
		ulog.Success("Wrote section").
			Field("section", ss.section.Name).
			Field("path", outputPath).
			Emit()
	}

	if len(failedSections) > 0 {
		return g.failedSectionsError(failedSections)
	}
	return nil
}
