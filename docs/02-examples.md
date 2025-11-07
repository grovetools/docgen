Here are three practical examples for using `grove-docgen`, demonstrating increasing complexity from a basic setup to advanced configuration and full ecosystem integration.

### Example 1: Basic Setup

This example covers initializing `grove-docgen` in a new project and generating a simple set of documentation.

#### Configuration (`docs/docgen.config.yml`)

First, run `docgen init` to create the initial configuration and prompt files. The resulting `docgen.config.yml` will look similar to this:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json
enabled: true
title: My Go Library
description: A brief description of what this library does.
category: Libraries
settings:
  model: gemini-1.5-flash-latest
  output_dir: docs
sections:
  - name: introduction
    title: Introduction
    order: 10
    prompt: prompts/introduction.md
    output: introduction.md
  - name: usage
    title: Usage
    order: 20
    prompt: prompts/usage.md
    output: usage.md
```

#### Sample Prompts

The `docgen init` command creates starter prompts.

**`docs/prompts/introduction.md`:**
```markdown
Based on the provided project context, write a concise introduction to this project. Explain its primary purpose and the problem it solves.
```

**`docs/prompts/usage.md`:**
```markdown
Based on the provided project context, generate a basic usage guide. Include a simple code example demonstrating how to use the core functionality.
```

#### Commands

1.  **Initialize Configuration:**
    ```sh
    docgen init
    ```
2.  **Generate Documentation:**
    ```sh
    docgen generate
    ```
    This command reads `docs/docgen.config.yml`, builds project context by running `grove cx generate` internally, sends each prompt to the LLM, and saves the results.

#### Expected Output Structure

After running the commands, your project will have the following structure:

```
.
├── docs/
│   ├── docgen.config.yml
│   ├── introduction.md      # Generated documentation
│   ├── usage.md             # Generated documentation
│   └── prompts/
│       ├── introduction.md
│       └── usage.md
└── go.mod
```

### Example 2: Advanced Configuration

This example demonstrates a more complex setup, including README synchronization, structured JSON output, and documentation generated from a JSON schema.

#### Configuration (`docs/docgen.config.yml`)

This configuration adds a `readme` section, a `structured_output_file`, custom context rules, and a section for generating documentation from a schema.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json
enabled: true
title: Advanced CLI Tool
description: A CLI tool with advanced features and a well-defined configuration schema.
category: Tools
settings:
  model: gemini-1.5-pro-latest
  rules_file: docs.rules
  structured_output_file: docs/generated.json
  output_dir: docs/generated
readme:
  template: docs/README.md.tpl
  output: README.md
  source_section: introduction
  strip_lines: 1 # Strips the H1 title from the source file
sections:
  - name: introduction
    title: Introduction
    order: 10
    prompt: prompts/introduction.md
    output: introduction.md
    json_key: overview
  - name: installation
    title: Installation
    order: 20
    prompt: prompts/installation.md
    output: installation.md
  - name: configuration
    title: Configuration Reference
    order: 30
    type: schema_to_md
    source: schema/config.schema.json
    output: configuration.md
    model: gemini-1.5-flash-latest # Use a faster model for this structured task
```

#### Supporting Files

*   **`docs/docs.rules` (Context Rules):**
    ```
    # Include all source code
    **/*.go
    # Exclude tests
    !**/*_test.go
    # Include the schema file
    schema/config.schema.json
    ```
*   **`schema/config.schema.json` (Source Schema):**
    ```json
    {
      "$schema": "http://json-schema.org/draft-07/schema#",
      "title": "Tool Configuration",
      "description": "Configuration for the Advanced CLI Tool.",
      "type": "object",
      "properties": {
        "port": {
          "type": "integer",
          "description": "The network port to listen on."
        },
        "log_level": {
          "type": "string",
          "enum": ["debug", "info", "warn", "error"]
        }
      }
    }
    ```
*   **`docs/README.md.tpl` (README Template):**
    ```markdown
    # {{ .Title }}

    {{ .Description }}

    <!-- DOCGEN:INTRODUCTION:START -->
    <!-- Content will be injected here -->
    <!-- DOCGEN:INTRODUCTION:END -->

    ## More Information

    See the full documentation in the `docs/generated` directory.
    ```

#### Commands

1.  **Generate all documentation sections:**
    ```sh
    docgen generate
    ```
2.  **Synchronize the README:**
    ```sh
    docgen sync-readme
    ```
3.  **(Optional) Regenerate JSON from Markdown:** If you manually edit the Markdown files, you can update the JSON without calling the LLM again.
    ```sh
    docgen regen-json
    ```

#### Expected Output Structure

The process generates documentation, a structured JSON file, and updates the root `README.md`.

```
.
├── README.md                # Generated from template
├── docs/
│   ├── docgen.config.yml
│   ├── docs.rules
│   ├── README.md.tpl
│   ├── generated.json         # Structured JSON output
│   └── generated/
│       ├── introduction.md
│       ├── installation.md
│       └── configuration.md   # Generated from schema
└── schema/
    └── config.schema.json
```

### Example 3: Grove Ecosystem Integration

This example shows how `grove-docgen` operates in a monorepo (or "ecosystem") with multiple packages, aggregating all documentation into a central directory for a website.

#### Project Structure

```
.
├── grove.yml
├── Makefile
├── dist/                  # Aggregated output appears here
└── packages/
    ├── tool-a/
    │   ├── docs/
    │   │   └── docgen.config.yml
    │   └── ...
    └── lib-b/
        ├── docs/
        │   └── docgen.config.yml
        └── ...
```

#### Configuration

*   **`grove.yml` (Ecosystem Root):**
    ```yaml
    workspaces:
      - "packages/*"
    ```
*   **`packages/tool-a/docs/docgen.config.yml`:**
    ```yaml
    enabled: true
    title: Tool A
    description: The first tool in the ecosystem.
    category: Tools
    # ... sections ...
    ```
*   **`packages/lib-b/docs/docgen.config.yml`:**
    ```yaml
    enabled: true
    title: Library B
    description: A shared library used by other tools.
    category: Libraries
    # ... sections ...
    ```
*   **`Makefile` (Ecosystem Root):**
    ```makefile
    .PHONY: docs

    docs: generate-docs aggregate-docs

    generate-docs:
    	@echo "Generating documentation for all packages..."
    	grove ws foreach --no-private -- exec docgen generate

    aggregate-docs:
    	@echo "Aggregating all documentation..."
    	docgen aggregate -o dist
    ```

#### Commands

From the ecosystem root, a single command can generate and aggregate all documentation.

```sh
make docs
```

This workflow performs two main actions:
1.  `grove ws foreach ... exec docgen generate`: Runs `docgen generate` inside each package defined in `grove.yml`. This leverages `grove-context` to build context relevant to each specific package.
2.  `docgen aggregate -o dist`: Scans all workspace packages, finds the generated documentation for those with `enabled: true`, copies the artifacts into the `dist` directory, and creates a `manifest.json`.

#### Expected Output Structure

The `dist` directory contains all documentation, organized by package, with a manifest file that a static site generator can use to build navigation.

```
./dist/
├── manifest.json
├── tool-a/
│   ├── introduction.md
│   └── usage.md
└── lib-b/
    ├── introduction.md
    └── api-reference.md
```

**`dist/manifest.json` (Excerpt):**
```json
{
  "packages": [
    {
      "name": "tool-a",
      "title": "Tool A",
      "description": "The first tool in the ecosystem.",
      "category": "Tools",
      "docs_path": "./tool-a",
      "version": "v1.2.0",
      "sections": [
        { "title": "Introduction", "path": "./tool-a/introduction.md" },
        { "title": "Usage", "path": "./tool-a/usage.md" }
      ]
    },
    {
      "name": "lib-b",
      "title": "Library B",
      // ...
    }
  ],
  "generated_at": "..."
}
```