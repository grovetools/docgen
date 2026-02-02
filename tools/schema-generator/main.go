package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/grovetools/docgen/pkg/config"
)

func main() {
	r := &jsonschema.Reflector{
		AllowAdditionalProperties: true,
		ExpandedStruct:            true,
		FieldNameTag:              "yaml",
	}

	schema := r.Reflect(&config.DocgenConfig{})
	schema.Title = "Grove Docgen Configuration"
	schema.Description = "Configuration schema for grove-docgen documentation generation."

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling schema: %v", err)
	}

	// Write to schema directory
	if err := os.WriteFile("schema/docgen.config.schema.json", data, 0644); err != nil {
		log.Fatalf("Error writing schema file: %v", err)
	}

	log.Printf("Successfully generated docgen schema at schema/docgen.config.schema.json")
}
