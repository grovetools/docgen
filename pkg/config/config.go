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
}

// SettingsConfig holds generator-wide settings.
type SettingsConfig struct {
	Model                string `yaml:"model,omitempty"`
	RegenerationMode     string `yaml:"regeneration_mode,omitempty"`     // "scratch" or "reference"
	RulesFile            string `yaml:"rules_file,omitempty"`            // Custom rules file for cx generate
	StructuredOutputFile string `yaml:"structured_output_file,omitempty"` // Path for JSON output
	SystemPrompt         string `yaml:"system_prompt,omitempty"`         // Path to system prompt file or "default" to use built-in
}

// SectionConfig defines a single piece of documentation to be generated.
type SectionConfig struct {
	Name    string `yaml:"name"`
	Title   string `yaml:"title"`
	Order   int    `yaml:"order"`
	Prompt  string `yaml:"prompt"`   // Path to the LLM prompt file
	Output  string `yaml:"output"`   // Output markdown file
	JSONKey string `yaml:"json_key,omitempty"` // Key for structured JSON output
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