package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = "docgen.config.yml"

// DocgenConfig defines the structure for a package's documentation settings.
type DocgenConfig struct {
	Enabled     bool            `yaml:"enabled"`
	Title       string          `yaml:"title"`
	Description string          `yaml:"description"`
	Category    string          `yaml:"category"`
	Settings    SettingsConfig  `yaml:"settings,omitempty"`
	Sections    []SectionConfig `yaml:"sections"`
	Readme      *ReadmeConfig   `yaml:"readme,omitempty"`
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
	Model                string `yaml:"model,omitempty"`
	RegenerationMode     string `yaml:"regeneration_mode,omitempty"`     // "scratch" or "reference"
	RulesFile            string `yaml:"rules_file,omitempty"`            // Custom rules file for cx generate
	StructuredOutputFile string `yaml:"structured_output_file,omitempty"` // Path for JSON output
	SystemPrompt         string `yaml:"system_prompt,omitempty"`         // Path to system prompt file or "default" to use built-in
	OutputDir            string `yaml:"output_dir,omitempty"`            // Output directory for generated docs
	GenerationConfig     `yaml:",inline"`                                  // Global generation parameters
}

// SectionConfig defines a single piece of documentation to be generated.
type SectionConfig struct {
	Name             string `yaml:"name"`
	Title            string `yaml:"title"`
	Order            int    `yaml:"order"`
	Prompt           string `yaml:"prompt"`             // Path to the LLM prompt file
	Output           string `yaml:"output"`             // Output markdown file
	JSONKey          string `yaml:"json_key,omitempty"` // Key for structured JSON output
	Type             string `yaml:"type,omitempty"`     // Type of generation, e.g., "schema_to_md"
	Source           string `yaml:"source,omitempty"`   // Source file for generation, e.g., a schema file
	Model            string `yaml:"model,omitempty"`    // Per-section model override
	AggStripLines    int    `yaml:"agg_strip_lines,omitempty"` // Number of lines to strip from the top of the content during aggregation
	GenerationConfig `yaml:",inline"`                    // Per-section generation parameter overrides
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
	configPath := filepath.Join(dir, "docs", ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}

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