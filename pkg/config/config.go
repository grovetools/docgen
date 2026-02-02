package config

//go:generate sh -c "cd ../.. && go run ./tools/schema-generator/"

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
	Enabled     bool            `yaml:"enabled" jsonschema:"description=Whether documentation generation is enabled for this package"`
	Title       string          `yaml:"title" jsonschema:"description=Title of the package documentation"`
	Description string          `yaml:"description" jsonschema:"description=Brief description of the package"`
	Category    string          `yaml:"category" jsonschema:"description=Category for grouping in documentation sidebar"`
	Settings    SettingsConfig  `yaml:"settings,omitempty" jsonschema:"description=Generator-wide settings"`
	Sections    []SectionConfig `yaml:"sections" jsonschema:"description=List of documentation sections to generate"`
	Readme      *ReadmeConfig   `yaml:"readme,omitempty" jsonschema:"description=README synchronization configuration"`
	Sidebar     *SidebarConfig  `yaml:"sidebar,omitempty" jsonschema:"description=Website sidebar configuration"`
	Logos       []string        `yaml:"logos,omitempty" jsonschema:"description=Additional logo files to copy during aggregation (absolute paths with ~ expansion)"`
}

// SidebarConfig defines the sidebar ordering and display configuration.
// This is used by the grove-website to control how packages and categories
// are displayed in the documentation sidebar.
type SidebarConfig struct {
	CategoryOrder           []string                   `yaml:"category_order,omitempty" jsonschema:"description=Order of categories in sidebar"`
	Categories              map[string]SidebarCategory `yaml:"categories,omitempty" jsonschema:"description=Category configuration (icon, flat, packages order)"`
	Packages                map[string]SidebarPackage  `yaml:"packages,omitempty" jsonschema:"description=Package configuration (icon, color, status)"`
	PackageCategoryOverride map[string]string          `yaml:"package_category_override,omitempty" jsonschema:"description=Remap packages to different categories"`
}

// SidebarCategory defines configuration for a single category in the sidebar.
type SidebarCategory struct {
	Icon     string   `yaml:"icon,omitempty" jsonschema:"description=Nerd font icon for the category"`
	Flat     bool     `yaml:"flat,omitempty" jsonschema:"description=If true, show docs flat without package nesting"`
	Packages []string `yaml:"packages,omitempty" jsonschema:"description=Order of packages within this category"`
}

// SidebarPackage defines configuration for a single package in the sidebar.
type SidebarPackage struct {
	Icon   string `yaml:"icon,omitempty" jsonschema:"description=Nerd font icon for the package"`
	Color  string `yaml:"color,omitempty" jsonschema:"description=Color name (green, blue, cyan, etc.)"`
	Status string `yaml:"status,omitempty" jsonschema:"description=Publication status: draft, dev, or production,enum=draft,enum=dev,enum=production"`
}

// GenerationConfig holds LLM generation parameters
type GenerationConfig struct {
	Temperature     *float32 `yaml:"temperature,omitempty" jsonschema:"description=Controls randomness in generation (0.0-1.0),minimum=0,maximum=1"`
	TopP            *float32 `yaml:"top_p,omitempty" jsonschema:"description=Nucleus sampling parameter (0.0-1.0),minimum=0,maximum=1"`
	TopK            *int32   `yaml:"top_k,omitempty" jsonschema:"description=Top-k sampling parameter,minimum=1"`
	MaxOutputTokens *int32   `yaml:"max_output_tokens,omitempty" jsonschema:"description=Maximum length of generated content,minimum=1"`
}

// SettingsConfig holds generator-wide settings.
type SettingsConfig struct {
	Model                string   `yaml:"model,omitempty" jsonschema:"description=LLM model to use for generation"`
	OutputMode           string   `yaml:"output_mode,omitempty" jsonschema:"description=Output mode: package (default) or sections for website content,enum=package,enum=sections"`
	Ecosystems           []string `yaml:"ecosystems,omitempty" jsonschema:"description=List of ecosystem names to aggregate from"`
	RegenerationMode     string   `yaml:"regeneration_mode,omitempty" jsonschema:"description=Regeneration mode: scratch or reference,enum=scratch,enum=reference"`
	RulesFile            string   `yaml:"rules_file,omitempty" jsonschema:"description=Custom rules file for cx generate"`
	StructuredOutputFile string   `yaml:"structured_output_file,omitempty" jsonschema:"description=Path for JSON output"`
	SystemPrompt         string   `yaml:"system_prompt,omitempty" jsonschema:"description=Path to system prompt file or 'default' to use built-in"`
	OutputDir            string   `yaml:"output_dir,omitempty" jsonschema:"description=Output directory for generated docs"`
	GenerationConfig     `yaml:",inline"`
}

// SectionConfig defines a single piece of documentation to be generated.
type SectionConfig struct {
	Name             string             `yaml:"name" jsonschema:"description=Unique identifier for this section"`
	Title            string             `yaml:"title" jsonschema:"description=Display title for the section"`
	Order            int                `yaml:"order" jsonschema:"description=Order in which the section appears"`
	Schemas          []SchemaInput      `yaml:"schemas,omitempty" jsonschema:"description=List of schemas to aggregate into one page (for schema_to_md type)"`
	DocSources       []DocSectionSource `yaml:"doc_sources,omitempty" jsonschema:"description=Sources for pulling from generated package docs (for doc_sections type)"`
	Status           string             `yaml:"status,omitempty" jsonschema:"description=Publication status: draft, dev, or production (default: draft),enum=draft,enum=dev,enum=production"`
	Prompt           string             `yaml:"prompt,omitempty" jsonschema:"description=Path to the LLM prompt file"`
	Output           string             `yaml:"output" jsonschema:"description=Output markdown filename"`
	OutputDir        string             `yaml:"output_dir,omitempty" jsonschema:"description=Output directory name for sections mode"`
	JSONKey          string             `yaml:"json_key,omitempty" jsonschema:"description=Key for structured JSON output"`
	Type             string             `yaml:"type,omitempty" jsonschema:"description=Type of generation: schema_to_md, doc_sections, capture, or nb_concept (notebook concept to docs),enum=schema_to_md,enum=doc_sections,enum=capture,enum=nb_concept"`
	Source           string             `yaml:"source,omitempty" jsonschema:"description=Source identifier. For schema_to_md: path to JSON schema file (deprecated: use schemas instead). For nb_concept: concept ID (e.g. my-concept or workspace:my-concept for cross-workspace)"`
	Binary           string             `yaml:"binary,omitempty" jsonschema:"description=Binary name for capture type"`
	Format           string             `yaml:"format,omitempty" jsonschema:"description=Output format for capture type: styled (default) or plain,enum=styled,enum=plain"`
	Depth            int                `yaml:"depth,omitempty" jsonschema:"description=Recursion depth for capture type (default: 5)"`
	SubcommandOrder  []string           `yaml:"subcommand_order,omitempty" jsonschema:"description=Priority order for subcommands (rest alphabetical)"`
	Model            string             `yaml:"model,omitempty" jsonschema:"description=Per-section model override"`
	AggStripLines    int                `yaml:"agg_strip_lines,omitempty" jsonschema:"description=Number of lines to strip from the top during aggregation"`
	GenerationConfig `yaml:",inline"`
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
	Template      string      `yaml:"template" jsonschema:"description=Path to the README template, relative to package root"`
	Output        string      `yaml:"output" jsonschema:"description=Path to the output README file, relative to package root"`
	SourceSection string      `yaml:"source_section" jsonschema:"description=The name of the section to inject into the template"`
	StripLines    int         `yaml:"strip_lines,omitempty" jsonschema:"description=Number of lines to strip from the top of source file (default: 0)"`
	GenerateTOC   bool        `yaml:"generate_toc,omitempty" jsonschema:"description=Whether to generate a table of contents from sections"`
	BaseURL       string      `yaml:"base_url,omitempty" jsonschema:"description=Base URL for converting root-relative paths to absolute URLs"`
	Logo          *LogoConfig `yaml:"logo,omitempty" jsonschema:"description=Optional logo generation configuration"`
}

// LogoConfig defines settings for generating a combined logo+text SVG.
type LogoConfig struct {
	Input     string  `yaml:"input" jsonschema:"description=Path to input logo SVG (relative to base_url root or absolute)"`
	Output    string  `yaml:"output" jsonschema:"description=Path for output logo-with-text SVG"`
	Text      string  `yaml:"text" jsonschema:"description=Text to display below logo"`
	Font      string  `yaml:"font" jsonschema:"description=Path to TTF/OTF font file"`
	Color     string  `yaml:"color,omitempty" jsonschema:"description=Text color (hex), defaults to #589ac7"`
	Spacing   float64 `yaml:"spacing,omitempty" jsonschema:"description=Spacing between logo and text (default: 35)"`
	TextScale float64 `yaml:"text_scale,omitempty" jsonschema:"description=Text width as proportion of logo (default: 1.1)"`
	Width     float64 `yaml:"width,omitempty" jsonschema:"description=Output SVG width in pixels (default: 200)"`
}

// SchemaInput defines a single schema source when aggregating multiple schemas.
type SchemaInput struct {
	Path  string `yaml:"path" jsonschema:"description=Path to the schema file"`
	Title string `yaml:"title,omitempty" jsonschema:"description=Title for the H2 section"`
}

// DocSectionSource defines a source for pulling from generated package docs.
type DocSectionSource struct {
	Package     string   `yaml:"package" jsonschema:"description=Package name (must exist in configured ecosystems)"`
	Doc         string   `yaml:"doc,omitempty" jsonschema:"description=Explicit path to doc file (auto-discovers schema_to_md output if omitted)"`
	Title       string   `yaml:"title,omitempty" jsonschema:"description=H2 section title (e.g., User Configuration)"`
	Description string   `yaml:"description,omitempty" jsonschema:"description=Description of what this configuration context is for"`
	Properties  []string `yaml:"properties,omitempty" jsonschema:"description=Properties to document in this section (dot notation supported)"`
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