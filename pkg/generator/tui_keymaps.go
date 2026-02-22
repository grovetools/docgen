package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grovetools/docgen/pkg/config"
)

// TUIDescriptions holds LLM-generated descriptions for TUIs.
type TUIDescriptions struct {
	TUIs map[string]TUIDescription `json:"tuis"`
}

// TUIDescription holds descriptions for a single TUI.
type TUIDescription struct {
	Description  string            `json:"description"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Sections     map[string]string `json:"sections,omitempty"`
}

// TUIRegistryEntry describes a TUI's keybindings.
// This mirrors grove/pkg/keys/registry_generated.go types.
type TUIRegistryEntry struct {
	Name        string         `json:"Name"`
	Package     string         `json:"Package"`
	Description string         `json:"Description"`
	Sections    []SectionEntry `json:"Sections"`
}

// SectionEntry describes a keybinding section within a TUI.
type SectionEntry struct {
	Name     string         `json:"Name"`
	Bindings []BindingEntry `json:"Bindings"`
}

// BindingEntry describes a single keybinding.
type BindingEntry struct {
	Name        string   `json:"Name"`
	Keys        []string `json:"Keys"`
	Description string   `json:"Description"`
	Enabled     bool     `json:"Enabled"`
	ConfigKey   string   `json:"ConfigKey"`
}

// generateFromTUIKeymaps generates markdown documentation from TUI keybinding registry.
// This is a deterministic generator that doesn't require LLM calls.
func (g *Generator) generateFromTUIKeymaps(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating TUI keymaps: %s", section.Name)

	// Fetch registry from grove CLI
	cmd := exec.Command("grove", "keys", "dump")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to fetch TUI registry (ensure 'grove' is installed and built): %w\nOutput: %s", err, string(output))
	}

	var registry []TUIRegistryEntry
	if err := json.Unmarshal(output, &registry); err != nil {
		return fmt.Errorf("failed to parse TUI registry JSON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", section.Title))

	// Build a map of TUI name -> TUIEntry for looking up all config
	tuiConfigs := make(map[string]config.TUIEntry)
	for _, entry := range section.TUIs {
		tuiConfigs[entry.Name] = entry
	}

	// Load descriptions if specified
	var descriptions *TUIDescriptions
	if section.Descriptions != "" {
		descPath := filepath.Join(outputBaseDir, section.Descriptions)
		var err error
		descriptions, err = loadTUIDescriptions(descPath)
		if err != nil {
			g.logger.Warnf("Failed to load TUI descriptions from %s: %v", descPath, err)
		} else {
			g.logger.Infof("Loaded TUI descriptions from %s", descPath)
		}
	}

	// Determine which TUIs to document (preserving order from config)
	var targetTUIs []TUIRegistryEntry
	if len(section.TUIs) > 0 {
		// Explicit list provided - preserve order from config
		for _, tuiEntry := range section.TUIs {
			for _, entry := range registry {
				if entry.Name == tuiEntry.Name {
					targetTUIs = append(targetTUIs, entry)
					break
				}
			}
		}
	} else {
		// Auto-discover by package name (lowercase comparison)
		pkgName := strings.ToLower(cfg.Title)
		for _, entry := range registry {
			if entry.Package == pkgName {
				targetTUIs = append(targetTUIs, entry)
			}
		}
	}

	if len(targetTUIs) == 0 {
		g.logger.Warnf("No TUIs found matching package %s or specified TUIs list", cfg.Title)
		sb.WriteString("*No terminal UIs documented for this package yet.*\n")
	} else {
		for _, tui := range targetTUIs {
			sb.WriteString(fmt.Sprintf("## %s\n\n", tui.Name))

			// Use rich description if available, otherwise fall back to registry description
			tuiDesc := tui.Description
			var sectionDescs map[string]string
			var capabilities []string
			if descriptions != nil {
				if desc, ok := descriptions.TUIs[tui.Name]; ok {
					if desc.Description != "" {
						tuiDesc = desc.Description
					}
					sectionDescs = desc.Sections
					capabilities = desc.Capabilities
				}
			}
			if tuiDesc != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", tuiDesc))
			}

			// Show command and CLI docs link if specified
			tuiCfg, hasCfg := tuiConfigs[tui.Name]
			if hasCfg {
				if tuiCfg.Command != "" {
					sb.WriteString(fmt.Sprintf("**Command:** `%s`", tuiCfg.Command))
					if tuiCfg.CLIDocsURL != "" {
						sb.WriteString(fmt.Sprintf(" ([CLI Reference](%s))", tuiCfg.CLIDocsURL))
					}
					sb.WriteString("\n\n")
				}

				// Output media (asciinema, video, or screenshot)
				if tuiCfg.Asciinema != nil {
					sb.WriteString("```asciinema\n")
					sb.WriteString("{\n")
					sb.WriteString(fmt.Sprintf("  \"src\": \"%s\"", tuiCfg.Asciinema.Src))
					if tuiCfg.Asciinema.Poster != "" {
						sb.WriteString(fmt.Sprintf(",\n  \"poster\": \"%s\"", tuiCfg.Asciinema.Poster))
					}
					if tuiCfg.Asciinema.AutoPlay {
						sb.WriteString(",\n  \"autoPlay\": true")
					}
					if tuiCfg.Asciinema.Loop {
						sb.WriteString(",\n  \"loop\": true")
					}
					sb.WriteString("\n}\n```\n\n")
				} else if tuiCfg.Video != "" {
					sb.WriteString(fmt.Sprintf("![%s](%s#themed)\n\n", tui.Name, tuiCfg.Video))
				} else if tuiCfg.Screenshot != "" {
					if tuiCfg.ScreenshotDark != "" {
						sb.WriteString(fmt.Sprintf("![%s](%s#themed)\n\n", tui.Name, tuiCfg.Screenshot))
					} else {
						sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", tui.Name, tuiCfg.Screenshot))
					}
				}
			}

			// Output capabilities list below media
			if len(capabilities) > 0 {
				sb.WriteString("**Capabilities:**\n\n")
				for _, cap := range capabilities {
					sb.WriteString(fmt.Sprintf("- %s\n", cap))
				}
				sb.WriteString("\n")
			}

			for _, sec := range tui.Sections {
				// Verify there's at least one enabled binding in this section
				hasEnabled := false
				for _, b := range sec.Bindings {
					if b.Enabled {
						hasEnabled = true
						break
					}
				}
				if !hasEnabled {
					continue
				}

				sb.WriteString(fmt.Sprintf("### %s\n\n", sec.Name))

				// Add section description if available
				if sectionDescs != nil {
					if secDesc, ok := sectionDescs[sec.Name]; ok && secDesc != "" {
						sb.WriteString(fmt.Sprintf("%s\n\n", secDesc))
					}
				}

				sb.WriteString("| Action | Keybinding |\n")
				sb.WriteString("| :--- | :--- |\n")

				for _, b := range sec.Bindings {
					if !b.Enabled {
						continue
					}
					var keysFormatted []string
					for _, k := range b.Keys {
						keysFormatted = append(keysFormatted, fmt.Sprintf("`%s`", k))
					}
					desc := b.Description
					if desc == "" {
						desc = b.Name
					}
					sb.WriteString(fmt.Sprintf("| %s | %s |\n", desc, strings.Join(keysFormatted, ", ")))
				}
				sb.WriteString("\n")
			}

			// Append TOML configuration example block
			sb.WriteString(g.generateTUIConfigExample(tui))
		}
	}

	outputPath := filepath.Join(outputBaseDir, section.Output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write TUI keymaps output: %w", err)
	}

	g.logger.Infof("Successfully wrote TUI keymaps '%s' to %s", section.Name, outputPath)
	return nil
}

// generateTUIConfigExample generates a copy-pasteable TOML block for a TUI's keybindings.
func (g *Generator) generateTUIConfigExample(tui TUIRegistryEntry) string {
	var sb strings.Builder
	sb.WriteString("### Configuration\n\n")
	sb.WriteString("Override these keybindings in `grove.toml`:\n\n")
	sb.WriteString("```toml\n")

	// Calculate override path namespace (e.g. flow-status -> flow.status)
	shortName := strings.TrimPrefix(tui.Name, tui.Package+"-")
	sb.WriteString(fmt.Sprintf("[tui.keybindings.%s.%s]\n", tui.Package, shortName))

	for _, section := range tui.Sections {
		hasBindings := false
		for _, b := range section.Bindings {
			if b.Enabled {
				hasBindings = true
				break
			}
		}
		if !hasBindings {
			continue
		}

		sb.WriteString(fmt.Sprintf("# %s\n", section.Name))
		for _, binding := range section.Bindings {
			if !binding.Enabled {
				continue
			}
			cKey := binding.ConfigKey
			if cKey == "" {
				cKey = strings.ReplaceAll(strings.ToLower(binding.Name), " ", "_")
			}

			var quotedKeys []string
			for _, k := range binding.Keys {
				quotedKeys = append(quotedKeys, fmt.Sprintf("\"%s\"", k))
			}
			sb.WriteString(fmt.Sprintf("%s = [%s]\n", cKey, strings.Join(quotedKeys, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("```\n\n")
	return sb.String()
}

// generateTUIDescriptions uses LLM to generate rich descriptions for TUIs
// and saves them to a JSON file that can be used by tui_keymaps.
func (g *Generator) generateTUIDescriptions(packageDir string, section config.SectionConfig, cfg *config.DocgenConfig, outputBaseDir string) error {
	g.logger.Infof("Generating TUI descriptions: %s", section.Name)

	// Fetch registry from grove CLI
	cmd := exec.Command("grove", "keys", "dump")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to fetch TUI registry (ensure 'grove' is installed and built): %w", err)
	}

	var registry []TUIRegistryEntry
	if err := json.Unmarshal(output, &registry); err != nil {
		return fmt.Errorf("failed to parse TUI registry JSON: %w", err)
	}

	// Determine which TUIs to describe
	var targetTUIs []TUIRegistryEntry
	if len(section.TUIs) > 0 {
		for _, tuiEntry := range section.TUIs {
			for _, entry := range registry {
				if entry.Name == tuiEntry.Name {
					targetTUIs = append(targetTUIs, entry)
					break
				}
			}
		}
	} else {
		// Auto-discover by package name
		pkgName := strings.ToLower(cfg.Title)
		for _, entry := range registry {
			if entry.Package == pkgName {
				targetTUIs = append(targetTUIs, entry)
			}
		}
	}

	if len(targetTUIs) == 0 {
		g.logger.Warnf("No TUIs found for descriptions")
		return nil
	}

	// Setup rules file if specified
	if section.RulesFile != "" {
		if err := g.setupRulesFile(packageDir, section.RulesFile); err != nil {
			g.logger.WithError(err).Warnf("Failed to setup rules file %s", section.RulesFile)
		}
	}

	// Build prompt for LLM with base system prompt for tone/style
	var promptBuilder strings.Builder
	promptBuilder.WriteString(DefaultSystemPrompt)
	promptBuilder.WriteString(`
Generate descriptions for each Terminal UI (TUI) below.
These descriptions will be used in documentation to help users understand each TUI's purpose.

For each TUI, provide:
1. A description explaining what the TUI does and typical use cases (2-4 sentences). Be descriptive but factual.
2. A "capabilities" list (4-8 items) enumerating specific actions the TUI supports, derived from the keybindings. Each item should be a short phrase (3-6 words).
3. Brief descriptions for each keybinding section (1 sentence each).

TUIs to describe:
`)

	for _, tui := range targetTUIs {
		promptBuilder.WriteString(fmt.Sprintf("\n## %s\n", tui.Name))
		promptBuilder.WriteString(fmt.Sprintf("Current description: %s\n", tui.Description))
		promptBuilder.WriteString("Sections:\n")
		for _, sec := range tui.Sections {
			promptBuilder.WriteString(fmt.Sprintf("- %s: ", sec.Name))
			var bindingNames []string
			for _, b := range sec.Bindings {
				if b.Enabled {
					bindingNames = append(bindingNames, b.Name)
				}
			}
			promptBuilder.WriteString(strings.Join(bindingNames, ", "))
			promptBuilder.WriteString("\n")
		}
	}

	promptBuilder.WriteString(`
Output format (JSON only, no markdown fences):
{
  "tuis": {
    "tui-name": {
      "description": "Description of what this TUI does and typical use cases...",
      "capabilities": [
        "Browse and filter items",
        "Execute batch operations",
        "View detailed logs"
      ],
      "sections": {
        "SectionName": "What this section provides..."
      }
    }
  }
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
	var descriptions TUIDescriptions
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

	g.logger.Infof("Successfully wrote TUI descriptions for %d TUIs to %s", len(descriptions.TUIs), outputPath)
	return nil
}

// loadTUIDescriptions loads TUI descriptions from a JSON file.
func loadTUIDescriptions(path string) (*TUIDescriptions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var descriptions TUIDescriptions
	if err := json.Unmarshal(data, &descriptions); err != nil {
		return nil, err
	}
	return &descriptions, nil
}
