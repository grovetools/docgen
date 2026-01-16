package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grovetools/docgen/pkg/config"
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
	Sections map[string]interface{} `json:"sections"`
}

// Section represents a parsed documentation section
type Section struct {
	Title       string       `json:"title"`
	Content     string       `json:"content"`
	Subsections []Subsection `json:"subsections,omitempty"`
	CodeBlocks  []string     `json:"code_blocks,omitempty"`
}

// Subsection represents a subsection within a documentation section
type Subsection struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	CodeBlocks []string `json:"code_blocks,omitempty"`
}

// GenerateJSON reads markdown files and generates structured JSON
func (p *Parser) GenerateJSON(packageDir string, cfg *config.DocgenConfig) error {
	if cfg.Settings.StructuredOutputFile == "" {
		p.logger.Debug("No structured output file configured, skipping JSON generation")
		return nil
	}

	p.logger.Info("Generating structured JSON from markdown files...")
	
	docs := &ParsedDocs{
		Sections: make(map[string]interface{}),
	}
	
	// Process each section dynamically
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
		
		// Use section name as key
		key := section.Name
		if section.JSONKey != "" {
			key = section.JSONKey
		}
		
		// Parse the markdown content into structured data
		parsedSection := p.parseSection(string(content), section.Title)
		docs.Sections[key] = parsedSection
		
		p.logger.Debugf("Parsed section '%s' with %d subsections", key, len(parsedSection.Subsections))
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

// parseSection parses a markdown section into structured data
func (p *Parser) parseSection(content string, sectionTitle string) Section {
	// Split into subsections based on ## headings
	markdownSections := p.splitIntoSections(content, "##")
	
	section := Section{
		Title: sectionTitle,
	}
	
	// Extract main content (everything before first ## heading)
	var mainContent []string
	var codeBlocks []string
	var inCodeBlock bool
	var currentCodeBlock []string
	
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Stop at first ## heading
		if strings.HasPrefix(line, "## ") {
			break
		}
		
		// Skip the main # heading
		if strings.HasPrefix(line, "# ") {
			continue
		}
		
		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				codeBlocks = append(codeBlocks, strings.Join(currentCodeBlock, "\n"))
				currentCodeBlock = nil
				inCodeBlock = false
			} else {
				// Start of code block
				inCodeBlock = true
				currentCodeBlock = []string{}
			}
			continue
		}
		
		if inCodeBlock {
			currentCodeBlock = append(currentCodeBlock, line)
		} else {
			mainContent = append(mainContent, line)
		}
	}
	
	section.Content = strings.TrimSpace(strings.Join(mainContent, "\n"))
	section.CodeBlocks = codeBlocks
	
	// Parse subsections
	for _, mdSection := range markdownSections {
		if mdSection.Title == "" {
			continue
		}
		
		subsection := Subsection{
			Title:      mdSection.Title,
			Content:    mdSection.Content,
			CodeBlocks: mdSection.CodeBlocks,
		}
		section.Subsections = append(section.Subsections, subsection)
	}
	
	return section
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