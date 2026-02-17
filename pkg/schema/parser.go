package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Parser handles parsing JSON schemas.
type Parser struct {
	schemaData map[string]interface{}
}

// Property represents a schema property with extended metadata.
type Property struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Deprecated  bool        `json:"deprecated,omitempty"`
	Properties  []Property  `json:"properties,omitempty"`
	Items       *Property   `json:"items,omitempty"`

	// x-* Extensions
	Layer            string `json:"x-layer,omitempty"`
	Priority         int    `json:"x-priority,omitempty"`
	Important        bool   `json:"x-important,omitempty"`
	Sensitive        bool   `json:"x-sensitive,omitempty"`
	Hint             string `json:"x-hint,omitempty"`
	Status           string `json:"x-status,omitempty"`
	StatusMessage    string `json:"x-status-message,omitempty"`
	StatusSince      string `json:"x-status-since,omitempty"`
	StatusTarget     string `json:"x-status-target,omitempty"`
	StatusReplacedBy string `json:"x-status-replaced-by,omitempty"`
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

// Parse returns the root properties defined in the schema as structured Property objects.
func (p *Parser) Parse() ([]Property, error) {
	// Extract required fields array
	var required []string
	if reqArray, ok := p.schemaData["required"].([]interface{}); ok {
		for _, r := range reqArray {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	if properties, ok := p.schemaData["properties"].(map[string]interface{}); ok {
		props := p.extractProperties(properties, required)
		// Propagate layer from parent to children that don't have their own
		propagateLayer(props, "")
		return props, nil
	}

	return nil, nil
}

// propagateLayer recursively propagates the layer value from parent to children
// that don't have their own layer specified.
func propagateLayer(props []Property, parentLayer string) {
	for i := range props {
		// If this property doesn't have a layer, inherit from parent
		if props[i].Layer == "" && parentLayer != "" {
			props[i].Layer = parentLayer
		}
		// Propagate to children using this property's layer (which may be inherited)
		if len(props[i].Properties) > 0 {
			propagateLayer(props[i].Properties, props[i].Layer)
		}
	}
}

func (p *Parser) extractProperties(rawProps map[string]interface{}, required []string) []Property {
	var props []Property

	// Build required lookup set
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}

	for key, val := range rawProps {
		rawProp, ok := val.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle $ref resolution
		if ref, ok := rawProp["$ref"].(string); ok {
			resolved := p.resolveRef(ref)
			if resolved != nil {
				// Merge resolved props with rawProp (rawProp takes precedence for overrides)
				merged := make(map[string]interface{})
				for k, v := range resolved {
					merged[k] = v
				}
				for k, v := range rawProp {
					merged[k] = v
				}
				rawProp = merged
			}
		}

		prop := Property{
			Name:        key,
			Type:        getString(rawProp, "type"),
			Description: getString(rawProp, "description"),
			Required:    requiredSet[key],
			Default:     rawProp["default"],
			Deprecated:  getBool(rawProp, "deprecated"),

			// x-* Extensions
			Layer:            getString(rawProp, "x-layer"),
			Priority:         getInt(rawProp, "x-priority"),
			Important:        getBool(rawProp, "x-important"),
			Sensitive:        getBool(rawProp, "x-sensitive"),
			Hint:             getString(rawProp, "x-hint"),
			Status:           getString(rawProp, "x-status"),
			StatusMessage:    getString(rawProp, "x-status-message"),
			StatusSince:      getString(rawProp, "x-status-since"),
			StatusTarget:     getString(rawProp, "x-status-target"),
			StatusReplacedBy: getString(rawProp, "x-status-replaced-by"),
		}

		// Handle nested objects
		if prop.Type == "object" {
			if nestedRaw, ok := rawProp["properties"].(map[string]interface{}); ok {
				var nestedReq []string
				if reqArray, ok := rawProp["required"].([]interface{}); ok {
					for _, r := range reqArray {
						if s, ok := r.(string); ok {
							nestedReq = append(nestedReq, s)
						}
					}
				}
				prop.Properties = p.extractProperties(nestedRaw, nestedReq)
			} else if addProps, ok := rawProp["additionalProperties"].(map[string]interface{}); ok {
				// Handle map types: additionalProperties defines the value schema
				// Resolve $ref if present
				if ref, ok := addProps["$ref"].(string); ok {
					resolved := p.resolveRef(ref)
					if resolved != nil {
						addProps = resolved
					}
				}
				// Extract properties from the additionalProperties schema
				if addPropsProps, ok := addProps["properties"].(map[string]interface{}); ok {
					var addPropsReq []string
					if reqArray, ok := addProps["required"].([]interface{}); ok {
						for _, r := range reqArray {
							if s, ok := r.(string); ok {
								addPropsReq = append(addPropsReq, s)
							}
						}
					}
					prop.Properties = p.extractProperties(addPropsProps, addPropsReq)
				}
			}
		} else if prop.Type == "array" {
			if itemsRaw, ok := rawProp["items"].(map[string]interface{}); ok {
				// Resolve $ref if present
				if ref, ok := itemsRaw["$ref"].(string); ok {
					resolved := p.resolveRef(ref)
					if resolved != nil {
						itemsRaw = resolved
					}
				}
				// Extract properties from items schema if it's an object
				if itemsType, ok := itemsRaw["type"].(string); ok && itemsType == "object" {
					if itemsProps, ok := itemsRaw["properties"].(map[string]interface{}); ok {
						var itemsReq []string
						if reqArray, ok := itemsRaw["required"].([]interface{}); ok {
							for _, r := range reqArray {
								if s, ok := r.(string); ok {
									itemsReq = append(itemsReq, s)
								}
							}
						}
						prop.Properties = p.extractProperties(itemsProps, itemsReq)
					}
				} else {
					// Non-object items: create a simple item property
					itemProps := p.extractProperties(map[string]interface{}{"item": itemsRaw}, nil)
					if len(itemProps) > 0 {
						item := itemProps[0]
						prop.Items = &item
					}
				}
			}
		}

		props = append(props, prop)
	}

	// Sort by priority (ascending), then name
	sort.Slice(props, func(i, j int) bool {
		// Default priority to 999 if 0
		p1 := props[i].Priority
		if p1 == 0 {
			p1 = 999
		}

		p2 := props[j].Priority
		if p2 == 0 {
			p2 = 999
		}

		if p1 != p2 {
			return p1 < p2
		}
		return props[i].Name < props[j].Name
	})

	return props
}

// Helper functions for type-safe extraction
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

// RenderAsText converts the loaded schema into a plain text representation with x-* extensions.
func (p *Parser) RenderAsText() (string, error) {
	props, err := p.Parse()
	if err != nil {
		return "", err
	}

	var builder strings.Builder

	if title, ok := p.schemaData["title"].(string); ok {
		builder.WriteString(fmt.Sprintf("Schema Title: %s\n", title))
	}
	if description, ok := p.schemaData["description"].(string); ok {
		builder.WriteString(fmt.Sprintf("Schema Description: %s\n", description))
	}
	builder.WriteString("\n")

	p.renderPropertiesText(&builder, props, 0)

	return builder.String(), nil
}

func (p *Parser) renderPropertiesText(builder *strings.Builder, props []Property, indentLevel int) {
	indent := strings.Repeat("  ", indentLevel)

	for _, prop := range props {
		builder.WriteString(fmt.Sprintf("%s- Property: `%s`\n", indent, prop.Name))
		builder.WriteString(fmt.Sprintf("%s  - Type: %s\n", indent, prop.Type))

		// Enhanced Metadata - x-* extensions
		if prop.Layer != "" {
			builder.WriteString(fmt.Sprintf("%s  - Layer: %s\n", indent, prop.Layer))
		}
		if prop.Status != "" {
			builder.WriteString(fmt.Sprintf("%s  - Status: %s\n", indent, strings.ToUpper(prop.Status)))
			if prop.StatusMessage != "" {
				builder.WriteString(fmt.Sprintf("%s  - Notice: %s\n", indent, prop.StatusMessage))
			}
			if prop.StatusSince != "" {
				builder.WriteString(fmt.Sprintf("%s  - Since: %s\n", indent, prop.StatusSince))
			}
			if prop.StatusTarget != "" {
				builder.WriteString(fmt.Sprintf("%s  - Target: %s\n", indent, prop.StatusTarget))
			}
			if prop.StatusReplacedBy != "" {
				builder.WriteString(fmt.Sprintf("%s  - Replaced By: %s\n", indent, prop.StatusReplacedBy))
			}
		}
		if prop.Deprecated {
			builder.WriteString(fmt.Sprintf("%s  - Deprecated: true\n", indent))
		}
		if prop.Important {
			builder.WriteString(fmt.Sprintf("%s  - Wizard: true (Common Setup)\n", indent))
		}
		if prop.Sensitive {
			builder.WriteString(fmt.Sprintf("%s  - Sensitive: true\n", indent))
		}
		if prop.Priority > 0 {
			builder.WriteString(fmt.Sprintf("%s  - Priority: %d\n", indent, prop.Priority))
		}

		builder.WriteString(fmt.Sprintf("%s  - Required: %v\n", indent, prop.Required))

		if prop.Description != "" {
			builder.WriteString(fmt.Sprintf("%s  - Description: %s\n", indent, prop.Description))
		}
		if prop.Hint != "" {
			builder.WriteString(fmt.Sprintf("%s  - Hint: %s\n", indent, prop.Hint))
		}
		if prop.Default != nil {
			builder.WriteString(fmt.Sprintf("%s  - Default: %v\n", indent, prop.Default))
		}

		// Nested handling
		if prop.Type == "object" && len(prop.Properties) > 0 {
			p.renderPropertiesText(builder, prop.Properties, indentLevel+2)
		} else if prop.Type == "array" && prop.Items != nil {
			builder.WriteString(fmt.Sprintf("%s  - Items:\n", indent))
			p.renderPropertiesText(builder, []Property{*prop.Items}, indentLevel+2)
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
