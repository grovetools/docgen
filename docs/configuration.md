# Grove Docgen Configuration Guide

The `grove-docgen` tool is configured using a single YAML file, `docgen.config.yml`, which defines every aspect of the documentation generation process for a package. This guide covers all available options, from basic setup to advanced fine-tuning.

## File Location

The configuration file must be named `docgen.config.yml` and placed inside a `docs/` directory at the root of your package.

```
my-package/
├── docs/
│   ├── docgen.config.yml   <-- Configuration file
│   └── prompts/
│       ├── introduction.md
│       └── ...
├── go.mod
└── ...
```

---

## Top-Level Configuration

These are the primary fields that describe your package's documentation.

| Field         | Type    | Description                                                              |
|---------------|---------|--------------------------------------------------------------------------|
| `enabled`     | boolean | A master switch to enable or disable documentation generation for this package. |
| `title`       | string  | The display title for the documentation set (e.g., "Grove Tend").        |
| `description` | string  | A brief, one-sentence summary of the package's purpose.                  |
| `category`    | string  | A category used to group packages in the final documentation site (e.g., "Core Libraries", "Tools"). |

### Example

```yaml
# docs/docgen.config.yml
enabled: true
title: "Grove Tend"
description: "A Go library for creating powerful, scenario-based end-to-end testing frameworks."
category: "Core Libraries"
```

---

## Global Settings (`settings`)

The `settings` block contains global configuration that applies to all documentation sections unless overridden.

| Field                    | Type   | Description                                                                                                                              |
|--------------------------|--------|------------------------------------------------------------------------------------------------------------------------------------------|
| `model`                  | string | The LLM model to use for generation (e.g., `gemini-1.5-flash-latest`, `gemini-1.5-pro-latest`).                                            |
| `regeneration_mode`      | string | Determines how to handle existing documentation. `scratch` (default) generates from scratch. `reference` provides the old content as context for the new generation. |
| `rules_file`             | string | Path to a custom rules file (relative to `docs/`) for `cx` to gather specific code context.                                               |
| `structured_output_file` | string | If specified, `docgen` will parse the generated markdown and create a structured JSON file at this path (e.g., `pkg/docs/tend-docs.json`). |
| `system_prompt`          | string | Use `default` for built-in style guidelines, or provide a path to a custom system prompt file (relative to `docs/`).                      |

### Example

```yaml
settings:
  model: "gemini-1.5-pro-latest"
  regeneration_mode: "scratch"
  rules_file: "docs.rules"
  structured_output_file: "pkg/docs/tend-docs.json"
  system_prompt: "default"
```

---

## Generation Parameters

These parameters fine-tune the behavior of the LLM. They can be set globally within the `settings` block and can also be overridden for each individual section.

| Parameter           | Type    | Description                                                                                             |
|---------------------|---------|---------------------------------------------------------------------------------------------------------|
| `temperature`       | number  | Controls randomness (0.0-1.0). Lower values are more deterministic; higher values are more creative.      |
| `top_p`             | number  | Nucleus sampling threshold. The model considers tokens until their cumulative probability reaches this value. |
| `top_k`             | integer | Limits the model to consider only the top K most likely tokens at each step.                               |
| `max_output_tokens` | integer | The maximum length of the generated content in tokens. Useful for controlling the length of sections.     |

### Example (Global Setting)

```yaml
settings:
  model: "gemini-1.5-flash-latest"
  # Global generation parameters
  temperature: 0.7
  max_output_tokens: 4096
```

---

## Defining Sections (`sections`)

The `sections` block is an array where you define each piece of documentation to be generated. The order of sections in the final output is determined by the `order` field.

| Field      | Type    | Description                                                                                             |
|------------|---------|---------------------------------------------------------------------------------------------------------|
| `name`     | string  | A unique, machine-readable identifier for the section (e.g., `core-concepts`).                          |
| `title`    | string  | The human-readable title for the section (e.g., "Core Concepts").                                       |
| `order`    | integer | A number that determines the sorting order of the sections.                                             |
| `prompt`   | string  | The path to the prompt markdown file for this section, relative to the `docs/` directory.               |
| `output`   | string  | The filename for the generated markdown output.                                                         |
| `json_key` | string  | (Optional) The key to use for this section's content when generating structured JSON output. Defaults to `name`. |

### Example Section Definition

```yaml
sections:
  - name: "introduction"
    title: "Introduction"
    order: 1
    prompt: "prompts/introduction.prompt.md"
    output: "introduction.md"
    json_key: "introduction"
  - name: "core-concepts"
    title: "Core Concepts"
    order: 2
    prompt: "prompts/core-concepts.prompt.md"
    output: "core-concepts.md"
    json_key: "core_concepts"
```

---

## Per-Section Overrides

For greater control, you can override the global `model` and any generation parameters on a per-section basis. This is useful when a specific section requires a more capable model or different generation settings.

### Example with Overrides

In this example, the "Best Practices" section uses a more advanced model and a lower temperature for more deterministic, factual output compared to the global settings.

```yaml
settings:
  model: "gemini-1.5-flash-latest"
  temperature: 0.7

sections:
  - name: "introduction"
    title: "Introduction"
    order: 1
    prompt: "prompts/introduction.prompt.md"
    output: "introduction.md"
    # This section uses the global settings

  - name: "best-practices"
    title: "Best Practices"
    order: 4
    prompt: "prompts/best-practices.prompt.md"
    output: "best-practices.md"
    # Overrides for this section
    model: "gemini-1.5-pro-latest" # Use a more powerful model
    temperature: 0.3               # Make the output more focused
    max_output_tokens: 8192        # Allow for more detailed content
```

---

## Complete Examples

### Minimal Configuration

This is a basic configuration suitable for a small library.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json
enabled: true
title: "My Utility Library"
description: "A small Go library for common tasks."
category: "Utilities"
settings:
  model: "gemini-1.5-flash-latest"
  system_prompt: "default"
sections:
  - name: "introduction"
    title: "Introduction"
    order: 1
    prompt: "prompts/introduction.md"
    output: "introduction.md"
  - name: "usage"
    title: "Usage"
    order: 2
    prompt: "prompts/usage.md"
    output: "usage.md"
```

### Advanced Configuration

This example demonstrates a full-featured configuration using many of the available options, similar to the one used for `grove-tend`.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json
enabled: true
title: "Grove Tend"
description: "A Go library for creating powerful, scenario-based end-to-end testing frameworks."
category: "Core Libraries"
settings:
  model: "gemini-1.5-pro-latest"
  regeneration_mode: "scratch"
  rules_file: "docs.rules"
  structured_output_file: "pkg/docs/tend-docs.json"
  system_prompt: "default"
  temperature: 0.5
sections:
  - name: "introduction"
    title: "Introduction"
    order: 1
    prompt: "prompts/introduction.prompt.md"
    output: "introduction.md"
    json_key: "introduction"
  - name: "core-concepts"
    title: "Core Concepts"
    order: 2
    prompt: "prompts/core-concepts.prompt.md"
    output: "core-concepts.md"
    json_key: "core_concepts"
    # Use a slightly more deterministic setting for technical concepts
    temperature: 0.3
  - name: "usage-patterns"
    title: "Usage Patterns"
    order: 3
    prompt: "prompts/usage-patterns.prompt.md"
    output: "usage-patterns.md"
  - name: "best-practices"
    title: "Best Practices"
    order: 4
    prompt: "prompts/best-practices.prompt.md"
    output: "best-practices.md"
```

---

## Advanced Topics

### Using a Custom System Prompt

Instead of using the `"default"` system prompt, you can create your own file with tone and style guidelines.

1.  Create a file, e.g., `docs/prompts/custom-style.md`.
2.  Update your config: `system_prompt: "prompts/custom-style.md"`.

### Providing Custom Context with `rules_file`

The `rules_file` setting points to a file that tells the `cx` context engine which source files to include for the LLM. This is crucial for focusing the model on the most relevant code. The format is a simple list of file paths or glob patterns.

**Example `docs/docs.rules`:**

```
# Include all public Go files
pkg/**/*.go

# Exclude test files
!pkg/**/*_test.go

# Include the main README
../README.md
```

### Editor Integration with JSON Schema

To get autocompletion and validation for `docgen.config.yml` in editors like VS Code, add a schema comment to the top of your file.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json
enabled: true
# ... rest of your config
```