package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Parser handles parsing JSON schemas.
type Parser struct {
	schemaData map[string]interface{}
}

// NewParser creates a new schema parser.
func NewParser(schemaPath string) (*Parser, error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	var schemaData map[string]interface{}
	if err := json.Unmarshal(data, &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	return &Parser{schemaData: schemaData}, nil
}

// RenderAsText converts the loaded schema into a plain text representation.
func (p *Parser) RenderAsText() (string, error) {
	var builder strings.Builder

	if title, ok := p.schemaData["title"].(string); ok {
		builder.WriteString(fmt.Sprintf("Schema Title: %s\n", title))
	}
	if description, ok := p.schemaData["description"].(string); ok {
		builder.WriteString(fmt.Sprintf("Schema Description: %s\n", description))
	}
	builder.WriteString("\n")

	if properties, ok := p.schemaData["properties"].(map[string]interface{}); ok {
		p.renderProperties(&builder, properties, 0)
	}

	return builder.String(), nil
}

func (p *Parser) renderProperties(builder *strings.Builder, properties map[string]interface{}, indentLevel int) {
	indent := strings.Repeat("  ", indentLevel)
	for key, val := range properties {
		prop, ok := val.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle $ref
		if ref, ok := prop["$ref"].(string); ok {
			prop = p.resolveRef(ref)
			if prop == nil {
				continue
			}
		}

		propType, _ := prop["type"].(string)
		description, _ := prop["description"].(string)

		builder.WriteString(fmt.Sprintf("%s- Property: `%s`\n", indent, key))
		builder.WriteString(fmt.Sprintf("%s  - Type: %s\n", indent, propType))
		if description != "" {
			builder.WriteString(fmt.Sprintf("%s  - Description: %s\n", indent, description))
		}
		if defaultValue, ok := prop["default"]; ok {
			builder.WriteString(fmt.Sprintf("%s  - Default: %v\n", indent, defaultValue))
		}

		if propType == "object" {
			if nestedProps, ok := prop["properties"].(map[string]interface{}); ok {
				p.renderProperties(builder, nestedProps, indentLevel+2)
			}
		} else if propType == "array" {
			if items, ok := prop["items"].(map[string]interface{}); ok {
				builder.WriteString(fmt.Sprintf("%s  - Items:\n", indent))
				itemProps := map[string]interface{}{"item": items}
				p.renderProperties(builder, itemProps, indentLevel+2)
			}
		}
	}
}

func (p *Parser) resolveRef(ref string) map[string]interface{} {
	parts := strings.Split(ref, "/")
	if len(parts) < 2 || parts[0] != "#" {
		return nil // Only support local refs for now
	}

	var current interface{} = p.schemaData
	for _, part := range parts[1:] {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	if resolved, ok := current.(map[string]interface{}); ok {
		return resolved
	}

	return nil
}
