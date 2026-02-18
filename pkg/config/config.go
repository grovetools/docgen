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
	Enabled     bool            `yaml:"enabled" jsonschema:"description=Whether documentation generation is enabled for this package" jsonschema_extras:"x-layer=project,x-priority=10"`
	Title       string          `yaml:"title" jsonschema:"description=Title of the package documentation" jsonschema_extras:"x-layer=project,x-priority=11"`
	Description string          `yaml:"description" jsonschema:"description=Brief description of the package" jsonschema_extras:"x-layer=project,x-priority=12"`
	Category    string          `yaml:"category" jsonschema:"description=Category for grouping in documentation sidebar" jsonschema_extras:"x-layer=project,x-priority=15"`
	Settings    SettingsConfig  `yaml:"settings,omitempty" jsonschema:"description=Generator-wide settings" jsonschema_extras:"x-layer=project,x-priority=20"`
	Sections    []SectionConfig `yaml:"sections" jsonschema:"description=List of documentation sections to generate" jsonschema_extras:"x-layer=project,x-priority=30"`
	Readme      *ReadmeConfig   `yaml:"readme,omitempty" jsonschema:"description=README synchronization configuration" jsonschema_extras:"x-layer=project,x-priority=40"`
	Sidebar     *SidebarConfig  `yaml:"sidebar,omitempty" jsonschema:"description=Website sidebar configuration" jsonschema_extras:"x-layer=ecosystem,x-priority=50"`
	Logos       []string        `yaml:"logos,omitempty" jsonschema:"description=Additional logo files to copy during aggregation (absolute paths with ~ expansion)" jsonschema_extras:"x-layer=project,x-priority=45"`
}

// SidebarConfig defines the sidebar ordering and display configuration.
// This is used by the grove-website to control how packages and categories
// are displayed in the documentation sidebar.
type SidebarConfig struct {
	CategoryOrder           []string                   `yaml:"category_order,omitempty" jsonschema:"description=Order of categories in sidebar" jsonschema_extras:"x-layer=ecosystem,x-priority=50"`
	Categories              map[string]SidebarCategory `yaml:"categories,omitempty" jsonschema:"description=Category configuration (icon, flat, packages order)" jsonschema_extras:"x-layer=ecosystem,x-priority=51"`
	Packages                map[string]SidebarPackage  `yaml:"packages,omitempty" jsonschema:"description=Package configuration (icon, color, status)" jsonschema_extras:"x-layer=ecosystem,x-priority=52"`
	PackageCategoryOverride map[string]string          `yaml:"package_category_override,omitempty" jsonschema:"description=Remap packages to different categories" jsonschema_extras:"x-layer=ecosystem,x-priority=53"`
}

// SidebarCategory defines configuration for a single category in the sidebar.
type SidebarCategory struct {
	Icon     string   `yaml:"icon,omitempty" jsonschema:"description=Nerd font icon for the category" jsonschema_extras:"x-layer=ecosystem,x-priority=51"`
	Flat     bool     `yaml:"flat,omitempty" jsonschema:"description=If true, show docs flat without package nesting" jsonschema_extras:"x-layer=ecosystem,x-priority=52"`
	Packages []string `yaml:"packages,omitempty" jsonschema:"description=Order of packages within this category" jsonschema_extras:"x-layer=ecosystem,x-priority=53"`
}

// SidebarPackage defines configuration for a single package in the sidebar.
type SidebarPackage struct {
	Icon   string `yaml:"icon,omitempty" jsonschema:"description=Nerd font icon for the package" jsonschema_extras:"x-layer=ecosystem,x-priority=52"`
	Color  string `yaml:"color,omitempty" jsonschema:"description=Color name (green, blue, cyan, etc.)" jsonschema_extras:"x-layer=ecosystem,x-priority=53"`
	Status string `yaml:"status,omitempty" jsonschema:"description=Publication status: draft, dev, or production,enum=draft,enum=dev,enum=production" jsonschema_extras:"x-layer=ecosystem,x-priority=54"`
}

// GenerationConfig holds LLM generation parameters
type GenerationConfig struct {
	Temperature     *float32 `yaml:"temperature,omitempty" jsonschema:"description=Controls randomness in generation (0.0-1.0),minimum=0,maximum=1" jsonschema_extras:"x-layer=project,x-priority=25"`
	TopP            *float32 `yaml:"top_p,omitempty" jsonschema:"description=Nucleus sampling parameter (0.0-1.0),minimum=0,maximum=1" jsonschema_extras:"x-layer=project,x-priority=26"`
	TopK            *int32   `yaml:"top_k,omitempty" jsonschema:"description=Top-k sampling parameter,minimum=1" jsonschema_extras:"x-layer=project,x-priority=27"`
	MaxOutputTokens *int32   `yaml:"max_output_tokens,omitempty" jsonschema:"description=Maximum length of generated content,minimum=1" jsonschema_extras:"x-layer=project,x-priority=28"`
}

// SettingsConfig holds generator-wide settings.
type SettingsConfig struct {
	Model                string   `yaml:"model,omitempty" jsonschema:"description=LLM model to use for generation" jsonschema_extras:"x-layer=project,x-priority=20"`
	OutputMode           string   `yaml:"output_mode,omitempty" jsonschema:"description=Output mode: package (default) or sections for website content,enum=package,enum=sections" jsonschema_extras:"x-layer=project,x-priority=21"`
	Ecosystems           []string `yaml:"ecosystems,omitempty" jsonschema:"description=List of ecosystem names to aggregate from" jsonschema_extras:"x-layer=ecosystem,x-priority=22"`
	RegenerationMode     string   `yaml:"regeneration_mode,omitempty" jsonschema:"description=Regeneration mode: scratch or reference,enum=scratch,enum=reference" jsonschema_extras:"x-layer=project,x-priority=23"`
	RulesFile            string   `yaml:"rules_file,omitempty" jsonschema:"description=Custom rules file for cx generate" jsonschema_extras:"x-layer=project,x-priority=24"`
	StructuredOutputFile string   `yaml:"structured_output_file,omitempty" jsonschema:"description=Path for JSON output" jsonschema_extras:"x-layer=project,x-priority=29"`
	SystemPrompt         string   `yaml:"system_prompt,omitempty" jsonschema:"description=Path to system prompt file or 'default' to use built-in" jsonschema_extras:"x-layer=project,x-priority=25"`
	OutputDir            string   `yaml:"output_dir,omitempty" jsonschema:"description=Output directory for generated docs" jsonschema_extras:"x-layer=project,x-priority=26"`
	TocDepth             int      `yaml:"toc_depth,omitempty" jsonschema:"description=Maximum heading level to show in Table of Contents (default: 3)" jsonschema_extras:"x-layer=project,x-priority=27"`
	GenerationConfig     `yaml:",inline"`
}

// SectionConfig defines a single piece of documentation to be generated.
type SectionConfig struct {
	Name             string             `yaml:"name" jsonschema:"description=Unique identifier for this section" jsonschema_extras:"x-layer=project,x-priority=30"`
	Title            string             `yaml:"title" jsonschema:"description=Display title for the section" jsonschema_extras:"x-layer=project,x-priority=31"`
	Order            int                `yaml:"order" jsonschema:"description=Order in which the section appears" jsonschema_extras:"x-layer=project,x-priority=32"`
	Schemas          []SchemaInput      `yaml:"schemas,omitempty" jsonschema:"description=List of schemas to aggregate into one page (for schema_to_md type)" jsonschema_extras:"x-layer=project,x-priority=35"`
	DocSources       []DocSectionSource `yaml:"doc_sources,omitempty" jsonschema:"description=Sources for pulling from generated package docs (for doc_sections type)" jsonschema_extras:"x-layer=project,x-priority=36"`
	Status           string             `yaml:"status,omitempty" jsonschema:"description=Publication status: draft, dev, or production (default: draft),enum=draft,enum=dev,enum=production" jsonschema_extras:"x-layer=project,x-priority=33"`
	Prompt           string             `yaml:"prompt,omitempty" jsonschema:"description=Path to the LLM prompt file" jsonschema_extras:"x-layer=project,x-priority=37"`
	Output           string             `yaml:"output" jsonschema:"description=Output markdown filename" jsonschema_extras:"x-layer=project,x-priority=34"`
	OutputDir        string             `yaml:"output_dir,omitempty" jsonschema:"description=Output directory name for sections mode" jsonschema_extras:"x-layer=project,x-priority=34"`
	JSONKey          string             `yaml:"json_key,omitempty" jsonschema:"description=Key for structured JSON output" jsonschema_extras:"x-layer=project,x-priority=38"`
	Type             string             `yaml:"type,omitempty" jsonschema:"description=Type of generation: schema_to_md (LLM-generated), schema_table (deterministic table), schema_describe (generate descriptions JSON), schema_examples (generate example TOML snippets), doc_sections, capture, or nb_concept,enum=schema_to_md,enum=schema_table,enum=schema_describe,enum=schema_examples,enum=doc_sections,enum=capture,enum=nb_concept" jsonschema_extras:"x-layer=project,x-priority=30"`
	Source           string             `yaml:"source,omitempty" jsonschema:"description=Source identifier. For schema_to_md: path to JSON schema file (deprecated: use schemas instead). For nb_concept: concept ID (e.g. my-concept or workspace:my-concept for cross-workspace)" jsonschema_extras:"x-layer=project,x-priority=35"`
	Descriptions     string             `yaml:"descriptions,omitempty" jsonschema:"description=Path to JSON file with LLM-generated descriptions (for schema_table type)" jsonschema_extras:"x-layer=project,x-priority=39"`
	Examples         string             `yaml:"examples,omitempty" jsonschema:"description=Path to JSON file with LLM-generated examples (for schema_table type with format: json)" jsonschema_extras:"x-layer=project,x-priority=39"`
	ExamplesFormat   string             `yaml:"examples_format,omitempty" jsonschema:"description=Format of examples: toml (default) or yaml,enum=toml,enum=yaml" jsonschema_extras:"x-layer=project,x-priority=39"`
	Binary           string             `yaml:"binary,omitempty" jsonschema:"description=Binary name for capture type" jsonschema_extras:"x-layer=project,x-priority=36"`
	Format           string             `yaml:"format,omitempty" jsonschema:"description=Output format. For capture: styled (default) or plain. For schema_table: markdown (default) or json,enum=styled,enum=plain,enum=markdown,enum=json" jsonschema_extras:"x-layer=project,x-priority=37"`
	Depth            int                `yaml:"depth,omitempty" jsonschema:"description=Recursion depth for capture type (default: 5)" jsonschema_extras:"x-layer=project,x-priority=38"`
	SubcommandOrder  []string           `yaml:"subcommand_order,omitempty" jsonschema:"description=Priority order for subcommands (rest alphabetical)" jsonschema_extras:"x-layer=project,x-priority=39"`
	Model            string             `yaml:"model,omitempty" jsonschema:"description=Per-section model override" jsonschema_extras:"x-layer=project,x-priority=25"`
	RulesFile        string             `yaml:"rules_file,omitempty" jsonschema:"description=Path to a cx rules file for gathering context (for schema_describe and schema_examples types)" jsonschema_extras:"x-layer=project,x-priority=26"`
	AggStripLines    int                `yaml:"agg_strip_lines,omitempty" jsonschema:"description=Number of lines to strip from the top during aggregation" jsonschema_extras:"x-layer=project,x-priority=40"`
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
	Template      string      `yaml:"template" jsonschema:"description=Path to the README template, relative to package root" jsonschema_extras:"x-layer=project,x-priority=40"`
	Output        string      `yaml:"output" jsonschema:"description=Path to the output README file, relative to package root" jsonschema_extras:"x-layer=project,x-priority=41"`
	SourceSection string      `yaml:"source_section" jsonschema:"description=The name of the section to inject into the template" jsonschema_extras:"x-layer=project,x-priority=42"`
	StripLines    int         `yaml:"strip_lines,omitempty" jsonschema:"description=Number of lines to strip from the top of source file (default: 0)" jsonschema_extras:"x-layer=project,x-priority=45"`
	GenerateTOC   bool        `yaml:"generate_toc,omitempty" jsonschema:"description=Whether to generate a table of contents from sections" jsonschema_extras:"x-layer=project,x-priority=43"`
	BaseURL       string      `yaml:"base_url,omitempty" jsonschema:"description=Base URL for converting root-relative paths to absolute URLs" jsonschema_extras:"x-layer=project,x-priority=44"`
	Logo          *LogoConfig `yaml:"logo,omitempty" jsonschema:"description=Optional logo generation configuration" jsonschema_extras:"x-layer=project,x-priority=46"`
}

// LogoConfig defines settings for generating a combined logo+text SVG.
type LogoConfig struct {
	Input     string  `yaml:"input" jsonschema:"description=Path to input logo SVG (relative to base_url root or absolute)" jsonschema_extras:"x-layer=project,x-priority=46"`
	Output    string  `yaml:"output" jsonschema:"description=Path for output logo-with-text SVG" jsonschema_extras:"x-layer=project,x-priority=47"`
	Text      string  `yaml:"text" jsonschema:"description=Text to display below logo" jsonschema_extras:"x-layer=project,x-priority=48"`
	Font      string  `yaml:"font" jsonschema:"description=Path to TTF/OTF font file" jsonschema_extras:"x-layer=project,x-priority=49"`
	Color     string  `yaml:"color,omitempty" jsonschema:"description=Text color (hex), defaults to #589ac7" jsonschema_extras:"x-layer=project,x-priority=50"`
	Spacing   float64 `yaml:"spacing,omitempty" jsonschema:"description=Spacing between logo and text (default: 35)" jsonschema_extras:"x-layer=project,x-priority=51"`
	TextScale float64 `yaml:"text_scale,omitempty" jsonschema:"description=Text width as proportion of logo (default: 1.1)" jsonschema_extras:"x-layer=project,x-priority=52"`
	Width     float64 `yaml:"width,omitempty" jsonschema:"description=Output SVG width in pixels (default: 200)" jsonschema_extras:"x-layer=project,x-priority=53"`
}

// SchemaInput defines a single schema source when aggregating multiple schemas.
type SchemaInput struct {
	Path  string `yaml:"path" jsonschema:"description=Path to the schema file" jsonschema_extras:"x-layer=project,x-priority=35"`
	Title string `yaml:"title,omitempty" jsonschema:"description=Title for the H2 section" jsonschema_extras:"x-layer=project,x-priority=36"`
}

// DocSectionSource defines a source for pulling from generated package docs.
type DocSectionSource struct {
	Package     string   `yaml:"package" jsonschema:"description=Package name (must exist in configured ecosystems)" jsonschema_extras:"x-layer=project,x-priority=36"`
	Doc         string   `yaml:"doc,omitempty" jsonschema:"description=Explicit path to doc file (auto-discovers schema_to_md output if omitted)" jsonschema_extras:"x-layer=project,x-priority=37"`
	Title       string   `yaml:"title,omitempty" jsonschema:"description=H2 section title (e.g., User Configuration)" jsonschema_extras:"x-layer=project,x-priority=38"`
	Description string   `yaml:"description,omitempty" jsonschema:"description=Description of what this configuration context is for" jsonschema_extras:"x-layer=project,x-priority=39"`
	Properties  []string `yaml:"properties,omitempty" jsonschema:"description=Properties to document in this section (dot notation supported)" jsonschema_extras:"x-layer=project,x-priority=40"`
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