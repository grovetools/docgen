package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/sirupsen/logrus"
)

// Parser converts markdown documentation to structured JSON
type Parser struct {
	logger *logrus.Logger
}

// New creates a new parser instance
func New(logger *logrus.Logger) *Parser {
	return &Parser{logger: logger}
}

// MarkdownSection represents a parsed markdown section
type MarkdownSection struct {
	Title       string
	Content     string
	CodeBlocks  []string
}

// ParsedDocs represents the complete parsed documentation
type ParsedDocs struct {
	Introduction  string      `json:"introduction,omitempty"`
	CoreConcepts  []Concept   `json:"core_concepts,omitempty"`
	UsagePatterns []Pattern   `json:"usage_patterns,omitempty"`
	BestPractices []Practice  `json:"best_practices,omitempty"`
}

// Concept represents a core concept
type Concept struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// Pattern represents a usage pattern
type Pattern struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// Practice represents a best practice
type Practice struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// GenerateJSON reads markdown files and generates structured JSON
func (p *Parser) GenerateJSON(packageDir string, cfg *config.DocgenConfig) error {
	if cfg.Settings.StructuredOutputFile == "" {
		p.logger.Debug("No structured output file configured, skipping JSON generation")
		return nil
	}

	p.logger.Info("Generating structured JSON from markdown files...")
	
	docs := &ParsedDocs{}
	
	// Process each section based on its type
	for _, section := range cfg.Sections {
		mdPath := filepath.Join(packageDir, "docs", section.Output)
		
		// Check if markdown file exists
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			p.logger.Warnf("Markdown file %s does not exist, skipping", mdPath)
			continue
		}
		
		content, err := os.ReadFile(mdPath)
		if err != nil {
			p.logger.WithError(err).Errorf("Failed to read markdown file %s", mdPath)
			continue
		}
		
		// Parse based on section name or JSONKey
		key := section.JSONKey
		if key == "" {
			key = section.Name
		}
		
		switch key {
		case "introduction":
			docs.Introduction = p.parseIntroduction(string(content))
		case "core-concepts", "core_concepts":
			docs.CoreConcepts = p.parseConcepts(string(content))
		case "usage-patterns", "usage_patterns":
			docs.UsagePatterns = p.parsePatterns(string(content))
		case "best-practices", "best_practices":
			docs.BestPractices = p.parsePractices(string(content))
		default:
			p.logger.Warnf("Unknown section type: %s", key)
		}
	}
	
	// Write JSON output
	outputPath := filepath.Join(packageDir, cfg.Settings.StructuredOutputFile)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	jsonData, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}
	
	p.logger.Infof("Successfully wrote structured JSON to %s", outputPath)
	return nil
}

// parseIntroduction extracts the introduction text from markdown
func (p *Parser) parseIntroduction(content string) string {
	// Remove the first heading if present
	lines := strings.Split(content, "\n")
	var result []string
	
	for _, line := range lines {
		// Skip the main heading
		if strings.HasPrefix(line, "# ") {
			continue
		}
		result = append(result, line)
	}
	
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// parseConcepts extracts concepts from markdown sections
func (p *Parser) parseConcepts(content string) []Concept {
	sections := p.splitIntoSections(content, "###")
	var concepts []Concept
	
	for _, section := range sections {
		if section.Title == "" {
			continue
		}
		
		concept := Concept{
			Name:        section.Title,
			Description: section.Content,
			Example:     strings.Join(section.CodeBlocks, "\n"),
		}
		concepts = append(concepts, concept)
	}
	
	return concepts
}

// parsePatterns extracts patterns from markdown sections
func (p *Parser) parsePatterns(content string) []Pattern {
	sections := p.splitIntoSections(content, "##")
	var patterns []Pattern
	
	for _, section := range sections {
		if section.Title == "" {
			continue
		}
		
		pattern := Pattern{
			Name:        section.Title,
			Description: section.Content,
			Example:     strings.Join(section.CodeBlocks, "\n"),
		}
		patterns = append(patterns, pattern)
	}
	
	return patterns
}

// parsePractices extracts practices from markdown sections
func (p *Parser) parsePractices(content string) []Practice {
	sections := p.splitIntoSections(content, "##")
	var practices []Practice
	
	for _, section := range sections {
		if section.Title == "" {
			continue
		}
		
		// Combine content and code blocks for practices
		text := section.Content
		if len(section.CodeBlocks) > 0 {
			text = text + "\n\n" + strings.Join(section.CodeBlocks, "\n\n")
		}
		
		practice := Practice{
			Title: section.Title,
			Text:  text,
		}
		practices = append(practices, practice)
	}
	
	return practices
}

// splitIntoSections splits markdown content into sections based on heading level
func (p *Parser) splitIntoSections(content string, headingPrefix string) []MarkdownSection {
	var sections []MarkdownSection
	var currentSection *MarkdownSection
	var inCodeBlock bool
	var codeBlock []string
	
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		
		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				if currentSection != nil {
					currentSection.CodeBlocks = append(currentSection.CodeBlocks, strings.Join(codeBlock, "\n"))
				}
				codeBlock = nil
				inCodeBlock = false
			} else {
				// Start of code block
				inCodeBlock = true
				codeBlock = []string{}
			}
			continue
		}
		
		if inCodeBlock {
			codeBlock = append(codeBlock, line)
			continue
		}
		
		// Check for section heading
		if strings.HasPrefix(line, headingPrefix + " ") {
			// Save previous section if exists
			if currentSection != nil {
				sections = append(sections, *currentSection)
			}
			
			// Start new section
			title := strings.TrimSpace(strings.TrimPrefix(line, headingPrefix))
			currentSection = &MarkdownSection{
				Title: title,
			}
			continue
		}
		
		// Add content to current section
		if currentSection != nil {
			if currentSection.Content != "" {
				currentSection.Content += "\n"
			}
			currentSection.Content += line
		}
	}
	
	// Save last section
	if currentSection != nil {
		sections = append(sections, *currentSection)
	}
	
	// Trim content for each section
	for i := range sections {
		sections[i].Content = strings.TrimSpace(sections[i].Content)
	}
	
	return sections
}