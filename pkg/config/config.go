package config

import (
	"fmt"
	"os"
	"path/filepath"

	coreConfig "github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	"gopkg.in/yaml.v3"
)

const (
	ConfigFileName = "docgen.config.yml"

	// Publication status values
	StatusDraft      = "draft"      // Only in notebook, not synced anywhere
	StatusDev        = "dev"        // Synced to dev website (from notebook)
	StatusProduction = "production" // Synced to repo (and prod website)
)

// DocgenConfig defines the structure for a package's documentation settings.
type DocgenConfig struct {
	Enabled     bool            `yaml:"enabled"`
	Title       string          `yaml:"title"`
	Description string          `yaml:"description"`
	Category    string          `yaml:"category"`
	Settings    SettingsConfig  `yaml:"settings,omitempty"`
	Sections    []SectionConfig `yaml:"sections"`
	Readme      *ReadmeConfig   `yaml:"readme,omitempty"`
	Sidebar     *SidebarConfig  `yaml:"sidebar,omitempty"` // Website sidebar configuration
}

// SidebarConfig defines the sidebar ordering and display configuration.
// This is used by the grove-website to control how packages and categories
// are displayed in the documentation sidebar.
type SidebarConfig struct {
	CategoryOrder           []string                       `yaml:"category_order,omitempty"`            // Order of categories in sidebar
	Categories              map[string]SidebarCategory     `yaml:"categories,omitempty"`                // Category config (icon, flat, packages order)
	Packages                map[string]SidebarPackage      `yaml:"packages,omitempty"`                  // Package config (icon, color, status)
	PackageCategoryOverride map[string]string              `yaml:"package_category_override,omitempty"` // Remap packages to different categories
}

// SidebarCategory defines configuration for a single category in the sidebar.
type SidebarCategory struct {
	Icon     string   `yaml:"icon,omitempty"`     // Nerd font icon
	Flat     bool     `yaml:"flat,omitempty"`     // If true, show docs flat (no package nesting)
	Packages []string `yaml:"packages,omitempty"` // Order of packages within this category
}

// SidebarPackage defines configuration for a single package in the sidebar.
type SidebarPackage struct {
	Icon   string `yaml:"icon,omitempty"`   // Nerd font icon
	Color  string `yaml:"color,omitempty"`  // Color name (green, blue, cyan, etc.)
	Status string `yaml:"status,omitempty"` // Publication status: draft | dev | production
}

// GenerationConfig holds LLM generation parameters
type GenerationConfig struct {
	Temperature     *float32 `yaml:"temperature,omitempty"`       // Controls randomness (0.0 - 1.0)
	TopP            *float32 `yaml:"top_p,omitempty"`             // Nucleus sampling parameter
	TopK            *int32   `yaml:"top_k,omitempty"`             // Top-k sampling parameter
	MaxOutputTokens *int32   `yaml:"max_output_tokens,omitempty"` // Maximum length of generated content
}

// SettingsConfig holds generator-wide settings.
type SettingsConfig struct {
	Model                string   `yaml:"model,omitempty"`
	OutputMode           string   `yaml:"output_mode,omitempty"`           // "package" (default) or "sections" for website content
	Ecosystems           []string `yaml:"ecosystems,omitempty"`            // List of ecosystem names to aggregate from (from global groves config)
	RegenerationMode     string   `yaml:"regeneration_mode,omitempty"`     // "scratch" or "reference"
	RulesFile            string   `yaml:"rules_file,omitempty"`            // Custom rules file for cx generate
	StructuredOutputFile string   `yaml:"structured_output_file,omitempty"` // Path for JSON output
	SystemPrompt         string   `yaml:"system_prompt,omitempty"`         // Path to system prompt file or "default" to use built-in
	OutputDir            string   `yaml:"output_dir,omitempty"`            // Output directory for generated docs
	GenerationConfig     `yaml:",inline"`                                    // Global generation parameters
}

// SectionConfig defines a single piece of documentation to be generated.
type SectionConfig struct {
	Name             string `yaml:"name"`
	Title            string `yaml:"title"`
	Order            int    `yaml:"order"`
	Status           string `yaml:"status,omitempty"`     // Publication status: "draft", "dev", "production" (default: "draft")
	Prompt           string `yaml:"prompt"`               // Path to the LLM prompt file
	Output           string `yaml:"output"`               // Output markdown file
	OutputDir        string `yaml:"output_dir,omitempty"` // For "sections" mode: output directory name
	JSONKey          string `yaml:"json_key,omitempty"`   // Key for structured JSON output
	Type             string `yaml:"type,omitempty"`       // Type of generation, e.g., "schema_to_md"
	Source           string `yaml:"source,omitempty"`     // Source file for generation, e.g., a schema file
	Model            string `yaml:"model,omitempty"`      // Per-section model override
	AggStripLines    int    `yaml:"agg_strip_lines,omitempty"` // Number of lines to strip from the top of the content during aggregation
	GenerationConfig `yaml:",inline"`                      // Per-section generation parameter overrides
}

// GetStatus returns the effective status for a section, defaulting to "draft" if not set.
// This means only sections with explicit status: dev or status: production will be included.
func (s *SectionConfig) GetStatus() string {
	if s.Status == "" {
		return StatusDraft
	}
	return s.Status
}

// ReadmeConfig defines the settings for synchronizing the README.md.
type ReadmeConfig struct {
	Template      string `yaml:"template"`       // Path to the README template, relative to package root.
	Output        string `yaml:"output"`         // Path to the output README file, relative to package root.
	SourceSection string `yaml:"source_section"` // The 'name' of the section to inject into the template.
	StripLines    int    `yaml:"strip_lines,omitempty"` // Number of lines to strip from the top of source file (default: 0).
	GenerateTOC   bool   `yaml:"generate_toc,omitempty"` // Whether to generate a table of contents from sections (default: false).
}

// Load attempts to load a docgen.config.yml file from a given directory's docs/ subdirectory.
func Load(dir string) (*DocgenConfig, error) {
	cfg, _, err := LoadWithNotebook(dir)
	return cfg, err
}

// LoadFromPath loads a docgen config from a specific file path.
func LoadFromPath(configPath string) (*DocgenConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	var config DocgenConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	return &config, nil
}

// LoadWithNotebook tries to load docgen config from notebook location first, then falls back to repo docs/.
// Returns the config, the path where it was found, and any error.
// The returned path indicates whether we're in "notebook mode" or "repo mode".
func LoadWithNotebook(repoDir string) (*DocgenConfig, string, error) {
	// 1. Try to resolve workspace node for repoDir
	node, err := workspace.GetProjectByPath(repoDir)
	if err == nil {
		// 2. Try notebook config path
		cfg, cfgErr := coreConfig.LoadDefault()
		if cfgErr == nil {
			locator := workspace.NewNotebookLocator(cfg)
			docgenDir, docgenErr := locator.GetDocgenDir(node)
			if docgenErr == nil {
				notebookConfigPath := filepath.Join(docgenDir, ConfigFileName)
				if _, statErr := os.Stat(notebookConfigPath); statErr == nil {
					// 3. Config exists in notebook, load it
					data, readErr := os.ReadFile(notebookConfigPath)
					if readErr != nil {
						return nil, "", fmt.Errorf("failed to read %s: %w", notebookConfigPath, readErr)
					}

					var config DocgenConfig
					if unmarshalErr := yaml.Unmarshal(data, &config); unmarshalErr != nil {
						return nil, "", fmt.Errorf("failed to parse %s: %w", notebookConfigPath, unmarshalErr)
					}

					return &config, notebookConfigPath, nil
				}
			}
		}
	}

	// 4. Fallback to repo docs/docgen.config.yml
	repoConfigPath := filepath.Join(repoDir, "docs", ConfigFileName)
	if _, err := os.Stat(repoConfigPath); os.IsNotExist(err) {
		return nil, "", os.ErrNotExist
	}

	data, err := os.ReadFile(repoConfigPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read %s: %w", repoConfigPath, err)
	}

	var config DocgenConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, "", fmt.Errorf("failed to parse %s: %w", repoConfigPath, err)
	}

	return &config, repoConfigPath, nil
}

// MergeGenerationConfig merges section-specific overrides with global defaults
func MergeGenerationConfig(global, section GenerationConfig) GenerationConfig {
	merged := GenerationConfig{}
	
	// Start with global settings
	if global.Temperature != nil {
		temp := *global.Temperature
		merged.Temperature = &temp
	}
	if global.TopP != nil {
		topP := *global.TopP
		merged.TopP = &topP
	}
	if global.TopK != nil {
		topK := *global.TopK
		merged.TopK = &topK
	}
	if global.MaxOutputTokens != nil {
		maxTokens := *global.MaxOutputTokens
		merged.MaxOutputTokens = &maxTokens
	}
	
	// Override with section-specific settings
	if section.Temperature != nil {
		merged.Temperature = section.Temperature
	}
	if section.TopP != nil {
		merged.TopP = section.TopP
	}
	if section.TopK != nil {
		merged.TopK = section.TopK
	}
	if section.MaxOutputTokens != nil {
		merged.MaxOutputTokens = section.MaxOutputTokens
	}
	
	return merged
}