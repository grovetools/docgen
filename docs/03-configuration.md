# Configuration

The `docs/docgen.config.yml` file is the central control panel for `grove-docgen`. It defines everything from the high-level properties of your documentation to the specific models, prompts, and context used for generating each section.

## File Structure Overview

A typical `docgen.config.yml` file is organized into root-level metadata, a `settings` block for global configuration, a `sections` array defining each document to be generated, and an optional `readme` block for synchronizing the main `README.md`.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json

# Root-level metadata for the documentation package.
enabled: true
title: "Grove Docgen"
description: "LLM-powered, workspace-aware documentation generator."
category: "Developer Tools"

# Global settings that apply to all sections unless overridden.
settings:
  model: gemini-1.5-flash-latest
  regeneration_mode: reference
  rules_file: docs.rules
  output_dir: docs
  structured_output_file: dist/docs.json
  system_prompt: default
  
  # Global generation parameters
  temperature: 0.7
  top_p: 0.9

# An array defining each documentation file to be generated.
sections:
  - name: overview
    order: 1
    title: Overview
    prompt: prompts/01-overview.md
    output: 01-overview.md
    json_key: introduction
    agg_strip_lines: 1 # Strip H1 heading during aggregation
    
    # Per-section override for the model
    model: gemini-1.5-pro-latest
    
    # Per-section override for generation parameters
    max_output_tokens: 4096

  - name: config-schema
    order: 2
    title: Configuration Schema
    type: schema_to_md
    source: schema/docgen.config.schema.json
    output: 02-config-schema.md

# Configuration for synchronizing the project's main README.md.
readme:
  generate_toc: true
  template: docs/README.md.tpl
  output: README.md
  source_section: overview
  strip_lines: 1
```

## Root-Level Fields

These fields define the overall properties of your documentation package, primarily used by the `docgen aggregate` command to build a manifest for a documentation website.

| Field | Type | Description |
| :--- | :--- | :--- |
| `enabled` | boolean | If `false`, the `docgen aggregate` command will skip this package. Defaults to `true`. |
| `title` | string | The primary title of the documentation set (e.g., "Grove Flow"). |
| `description` | string | A brief, one-sentence description of the project. |
| `category` | string | A category used to group related packages in an aggregated documentation site (e.g., "Developer Tools"). |

## The `settings` Section

This section contains global configurations that apply to all documentation sections unless overridden at the section level.

| Field | Type | Description |
| :--- | :--- | :--- |
| `model` | string | The default LLM model to use for generation (e.g., `gemini-1.5-flash-latest`, `gpt-4o`). |
| `regeneration_mode` | string | Determines regeneration behavior. `scratch` generates from scratch. `reference` provides the existing document to the LLM as context for updates. |
| `rules_file` | string | Path to the `grove-context` rules file (relative to the `docs/` directory) that defines the codebase context for the LLM. |
| `structured_output_file` | string | (Optional) Path to a file where structured JSON output will be saved after parsing the generated Markdown. |
| `system_prompt` | string | Can be set to `default` to use the built-in system prompt, or a path to a custom prompt file (relative to `docs/`). |
| `output_dir` | string | The directory where generated documentation files will be saved, relative to the project root. Defaults to `docs`. |

### Global Generation Parameters

You can set default LLM generation parameters within the `settings` block. These will be applied to every section unless a section provides its own override.

- `temperature` (float, 0.0-1.0): Controls randomness. Higher values produce more creative but less predictable output.
- `top_p` (float): Nucleus sampling parameter.
- `top_k` (integer): Top-k sampling parameter.
- `max_output_tokens` (integer): The maximum number of tokens to generate for each section.

## The `sections` Array

This is a list where each item represents a single Markdown file to be generated. The order of generation is determined by the `order` field.

| Field | Type | Description |
| :--- | :--- | :--- |
| `name` | string | A unique, internal identifier for the section (e.g., `overview`, `command-reference`). |
| `title` | string | The user-facing title for the section, often used as the H1 heading (e.g., "Command Reference"). |
| `order` | integer| A number that determines the sorting order of sections in the final aggregated manifest. |
| `prompt`| string | The path to the Markdown prompt file for this section, relative to the `docs/` directory. Required unless `type` is specified. |
| `output`| string | The path where the generated Markdown file will be saved, relative to the `output_dir` setting. |
| `json_key`| string | (Optional) A key used to identify this section's content when generating a structured JSON output file. |
| `type` | string | (Optional) Specifies a special generation type. Currently, only `schema_to_md` is supported. |
| `source` | string | (Optional) The source file for a special `type`. For `schema_to_md`, this is the path to the JSON schema file. |
| `agg_strip_lines` | integer | (Optional) Number of lines to remove from the top of the generated file during the `docgen aggregate` process. Useful for removing H1 titles. |

### Per-Section Overrides

You can override the global `model` and any generation parameters on a per-section basis. This is useful for tasks that require different models or more constrained outputs.

```yaml
sections:
  - name: overview
    # ...
    # This section uses the global model and settings.

  - name: command-reference
    # ...
    # This section uses a more powerful model and has a higher token limit.
    model: gemini-1.5-pro-latest
    max_output_tokens: 8192
```

### Special Section Types

#### `schema_to_md`
This type generates user-friendly Markdown documentation from a JSON schema file. It does not use a `prompt` file. Instead, it parses the `source` schema and uses a built-in prompt to ask the LLM to describe it.

```yaml
sections:
  - name: config-reference
    title: Configuration Reference
    order: 2
    type: schema_to_md
    source: schema/config.schema.json # Path to the JSON schema
    output: 02-config-reference.md
```

## The `readme` Section

This section configures the `docgen sync-readme` command, which generates the project's main `README.md` from a template.

| Field | Type | Description |
| :--- | :--- | :--- |
| `template` | string | Path to the README template file (e.g., `docs/README.md.tpl`). |
| `output` | string | Path to the final output `README.md` file. |
| `source_section`| string | The `name` of the section from the `sections` array whose content will be injected into the template. |
| `strip_lines` | integer | (Optional) Number of lines to remove from the top of the `source_section` content before injection. |
| `generate_toc` | boolean | (Optional) If `true`, a table of contents linking to all documentation files will be injected. |

## Advanced Topics

### Context Management with `rules_file`

The `rules_file` setting is a key integration point with `grove-context` (`cx`). The file specified here uses a `.gitignore`-style syntax to define which source code files from your project are collected and provided to the LLM as context. A well-defined rules file is critical for generating accurate, code-aware documentation.

### JSON Schema Validation

The `docgen.config.yml` file can be validated against a JSON schema. Including the schema line at the top of your file enables autocompletion and validation in compatible editors like VS Code, helping you avoid configuration errors.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json
```

### Configuration Precedence

Settings are applied with a clear order of precedence, allowing for fine-grained control:
1.  **Section-level**: A `model` or generation parameter (e.g., `temperature`) set directly within a `sections` item has the highest priority.
2.  **Global `settings`**: If not defined at the section level, the value from the main `settings` block is used.
3.  **Application Defaults**: If a setting is not found in the configuration, a hardcoded default within `grove-docgen` is used.