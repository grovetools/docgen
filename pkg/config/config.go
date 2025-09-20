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
	Model       string          `yaml:"model,omitempty"` // Optional LLM model to use
	Sections    []SectionConfig `yaml:"sections"`
}

// SectionConfig defines a single piece of documentation to be generated.
type SectionConfig struct {
	Name   string `yaml:"name"`
	Title  string `yaml:"title"`
	Order  int    `yaml:"order"`
	Prompt string `yaml:"prompt"` // Path to the LLM prompt file
	Output string `yaml:"output"` // Output markdown file
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