package schema_enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	grovelogging "github.com/mattsolo1/grove-core/logging"
	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/mattsolo1/grove-docgen/pkg/generator"
	"github.com/sirupsen/logrus"
)

var ulog = grovelogging.NewUnifiedLogger("grove-docgen.enricher")

// Enricher handles the process of enriching a JSON schema.
type Enricher struct {
	logger    *logrus.Logger
	generator *generator.Generator
}

// propertyInfo holds information about a property that needs a description
type propertyInfo struct {
	path   string
	schema map[string]interface{}
}

// New creates a new Enricher instance.
func New(logger *logrus.Logger) *Enricher {
	return &Enricher{
		logger:    logger,
		generator: generator.New(logger), // Reuse generator for its methods
	}
}

// Enrich finds properties without descriptions and generates them using an LLM.
func (e *Enricher) Enrich(projectDir, schemaPath string, inPlace bool) error {
	e.logger.Infof("Enriching schema: %s", schemaPath)

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	var schemaData map[string]interface{}
	if err := json.Unmarshal(data, &schemaData); err != nil {
		return fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	// Load docgen config to get rules file setting
	cfg, err := config.Load(projectDir)
	if err != nil {
		e.logger.Warnf("No docgen.config.yml found, using default context rules")
	} else if cfg.Settings.RulesFile != "" {
		// Setup rules file if specified in config
		e.logger.Infof("Setting up rules file: %s", cfg.Settings.RulesFile)
		if err := e.setupRulesFile(projectDir, cfg.Settings.RulesFile); err != nil {
			return fmt.Errorf("failed to setup rules file: %w", err)
		}
	}

	// Build context once for the entire project
	e.logger.Info("Building project context with 'cx generate'...")
	if err := e.generator.BuildContext(projectDir); err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	// Collect all properties that need descriptions
	propsNeedingDescriptions := e.collectPropertiesNeedingDescriptions(schemaData, "")

	// Add top-level schema description if missing
	if _, hasDesc := schemaData["description"]; !hasDesc {
		propsNeedingDescriptions = append([]propertyInfo{{
			path:   "_schema",
			schema: schemaData,
		}}, propsNeedingDescriptions...)
	}

	// Generate all descriptions in a single batch call
	if len(propsNeedingDescriptions) > 0 {
		e.logger.Infof("Generating descriptions for %d properties in batch...", len(propsNeedingDescriptions))
		descriptions, err := e.generateDescriptionsBatch(projectDir, propsNeedingDescriptions, cfg)
		if err != nil {
			return fmt.Errorf("failed to generate descriptions: %w", err)
		}

		// Apply the descriptions
		for i, propInfo := range propsNeedingDescriptions {
			if i < len(descriptions) {
				propInfo.schema["description"] = descriptions[i]
				e.logger.Infof("Updated description for: %s", propInfo.path)
			}
		}
	} else {
		e.logger.Info("All properties already have descriptions")
	}

	// Marshal the updated schema back to JSON
	updatedData, err := json.MarshalIndent(schemaData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated schema: %w", err)
	}

	ctx := context.Background()
	if inPlace {
		if err := os.WriteFile(schemaPath, updatedData, 0644); err != nil {
			return fmt.Errorf("failed to write updated schema file: %w", err)
		}
		e.logger.Infof("Successfully enriched schema in-place: %s", schemaPath)
	} else {
		ulog.Info("Enriched schema output").
			Field("schema_path", schemaPath).
			PrettyOnly().
			Pretty(string(updatedData)).
			Log(ctx)
	}

	return nil
}

func (e *Enricher) collectPropertiesNeedingDescriptions(node interface{}, path string) []propertyInfo {
	var results []propertyInfo

	switch v := node.(type) {
	case map[string]interface{}:
		if props, ok := v["properties"].(map[string]interface{}); ok {
			for key, val := range props {
				prop, ok := val.(map[string]interface{})
				if !ok {
					continue
				}
				newPath := path + "." + key
				if path == "" {
					newPath = key
				}

				// If no description, add to list
				if _, hasDesc := prop["description"]; !hasDesc {
					results = append(results, propertyInfo{
						path:   newPath,
						schema: prop,
					})
				}

				// Recurse for nested objects
				if nestedProps, ok := prop["properties"].(map[string]interface{}); ok {
					results = append(results, e.collectPropertiesNeedingDescriptions(nestedProps, newPath)...)
				}
			}
		}
	}
	return results
}

type enrichmentResult struct {
	Description      string            `json:"description"`
	Examples         []interface{}     `json:"examples,omitempty"`
	EnumDescriptions map[string]string `json:"enum_descriptions,omitempty"`
}

func (e *Enricher) generateDescriptionsBatch(projectDir string, properties []propertyInfo, cfg *config.DocgenConfig) ([]string, error) {
	// Build the batch prompt
	var promptBuilder strings.Builder
	promptBuilder.WriteString("Given the project context, enrich the following JSON schema properties.\n\n")
	promptBuilder.WriteString("For each property, generate:\n")
	promptBuilder.WriteString("1. A concise one-sentence description\n")
	promptBuilder.WriteString("2. Example values (1-3 realistic examples)\n")
	promptBuilder.WriteString("3. If the property has enum values, provide a description for each enum value\n\n")
	promptBuilder.WriteString("Return your response as a JSON array with one object per property, in the same order as listed below.\n")
	promptBuilder.WriteString("Each object should have:\n")
	promptBuilder.WriteString("- \"description\": string (the one-sentence description)\n")
	promptBuilder.WriteString("- \"examples\": array (1-3 example values, omit if not applicable)\n")
	promptBuilder.WriteString("- \"enum_descriptions\": object (map of enum value to description, omit if no enum)\n\n")
	promptBuilder.WriteString("---\n\n")

	for i, prop := range properties {
		propJSON, err := json.MarshalIndent(prop.schema, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal property %s: %w", prop.path, err)
		}

		if prop.path == "_schema" {
			promptBuilder.WriteString(fmt.Sprintf("Property %d: Top-level schema description\n", i+1))
			promptBuilder.WriteString(fmt.Sprintf("Schema Title: %v\n", prop.schema["title"]))
		} else {
			promptBuilder.WriteString(fmt.Sprintf("Property %d: %s\n", i+1, prop.path))
		}
		promptBuilder.WriteString(fmt.Sprintf("Schema:\n%s\n\n", string(propJSON)))
	}

	// Use model and generation config from docgen config if available
	model := ""
	genConfig := config.GenerationConfig{}
	if cfg != nil {
		model = cfg.Settings.Model
		genConfig = cfg.Settings.GenerationConfig
	}

	response, err := e.generator.CallLLM(promptBuilder.String(), model, genConfig, projectDir)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	response = strings.TrimSpace(response)
	// Remove markdown code fences if present
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	var results []enrichmentResult
	if err := json.Unmarshal([]byte(response), &results); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w\nResponse: %s", err, response)
	}

	if len(results) != len(properties) {
		e.logger.Warnf("Expected %d results but got %d. Using what we have.", len(properties), len(results))
	}

	// Apply enrichments to the actual schema objects
	for i, result := range results {
		if i >= len(properties) {
			break
		}
		prop := properties[i]

		// Add examples if provided and property doesn't have them
		if len(result.Examples) > 0 {
			if _, hasExamples := prop.schema["examples"]; !hasExamples {
				prop.schema["examples"] = result.Examples
				e.logger.Debugf("Added %d examples for: %s", len(result.Examples), prop.path)
			}
		}

		// Add enum descriptions if provided
		if len(result.EnumDescriptions) > 0 {
			if enumVals, hasEnum := prop.schema["enum"]; hasEnum {
				// Create an anyOf structure with const + description for each enum value
				var anyOf []map[string]interface{}
				if enumArray, ok := enumVals.([]interface{}); ok {
					for _, val := range enumArray {
						valStr := fmt.Sprintf("%v", val)
						enumObj := map[string]interface{}{
							"const": val,
						}
						if desc, hasDesc := result.EnumDescriptions[valStr]; hasDesc {
							enumObj["description"] = desc
						}
						anyOf = append(anyOf, enumObj)
					}
					// Only replace if we have descriptions
					if len(anyOf) > 0 {
						delete(prop.schema, "enum")
						prop.schema["anyOf"] = anyOf
						e.logger.Debugf("Added enum descriptions for: %s", prop.path)
					}
				}
			}
		}
	}

	// Extract just descriptions to return
	var descriptions []string
	for _, result := range results {
		descriptions = append(descriptions, result.Description)
	}

	return descriptions, nil
}

func (e *Enricher) setupRulesFile(packageDir, rulesFile string) error {
	// Read the specified rules file
	rulesPath := filepath.Join(packageDir, "docs", rulesFile)
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		return fmt.Errorf("failed to read rules file %s: %w", rulesPath, err)
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(packageDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("failed to create .grove directory: %w", err)
	}

	// Copy the rules file content to .grove/rules
	groveRulesPath := filepath.Join(groveDir, "rules")
	if err := os.WriteFile(groveRulesPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write .grove/rules: %w", err)
	}

	e.logger.Debugf("Setup rules file from %s to .grove/rules", rulesFile)
	return nil
}
