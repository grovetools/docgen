# Configuration

The `docs/docgen.config.yml` file is the central control panel for `grove-docgen`. It defines everything from the high-level properties of your documentation to the specific models, prompts, and context used for generating each section.

## File Structure Overview

A typical `docgen.config.yml` file is organized into root-level metadata, a `settings` block for global configuration, and a `sections` array defining each document to be generated.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json

# Root-level metadata for the documentation package.
enabled: true
title: "Grove Context"
description: "A rule-based tool for managing file-based context for LLMs."
category: "Developer Tools"

# Global settings that apply to all sections unless overridden.
settings:
  model: gemini-2.5-pro
  regeneration_mode: reference
  rules_file: docs.rules
  output_dir: docs
  structured_output_file: pkg/docs/docs.json
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
    
    # Per-section override for the model
    model: gemini-2.0-flash-latest
    
    # Per-section override for generation parameters
    max_output_tokens: 4096
```

## Root-Level Fields

These fields define the overall properties of your documentation package, primarily used when aggregating documentation from multiple projects.

| Field         | Type    | Description                                                                                             |
| :------------ | :------ | :------------------------------------------------------------------------------------------------------ |
| `enabled`     | boolean | If `false`, the `docgen aggregate` command will skip this package. Defaults to `true`.                  |
| `title`       | string  | The primary title of the documentation set (e.g., "Grove Flow").                                        |
| `description` | string  | A brief, one-sentence description of the project.                                                       |
| `category`    | string  | A category used to group related packages in an aggregated documentation site (e.g., "Developer Tools"). |

## The `settings` Section

This section contains global configurations that apply to all documentation sections.

| Field                    | Type   | Description                                                                                                                                                                                          |
| :----------------------- | :----- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `model`                  | string | The default LLM model to use for generation (e.g., `gemini-2.5-pro`, `gpt-4o`).                                                                                                                        |
| `regeneration_mode`      | string | Determines regeneration behavior. `scratch` generates from scratch. `reference` provides the existing document to the LLM as context for updates.                                                      |
| `rules_file`             | string | Path to the `grove-context` rules file (relative to the `docs/` directory) that defines the codebase context for the LLM.                                                                              |
| `structured_output_file` | string | (Optional) Path to a file where structured JSON output will be saved after parsing the generated Markdown.                                                                                             |
| `system_prompt`          | string | Can be set to `default` to use the built-in system prompt, or a path to a custom prompt file (relative to `docs/`).                                                                                    |
| `output_dir`             | string | The directory where generated documentation files will be saved, relative to the project root. Defaults to `docs`.                                                                                     |

### Global Generation Parameters

You can set default LLM generation parameters within the `settings` block. These will be applied to every section unless a section provides its own override.

-   `temperature` (float, 0.0-1.0): Controls randomness. Higher values produce more creative but less predictable output.
-   `top_p` (float): Nucleus sampling parameter.
-   `top_k` (integer): Top-k sampling parameter.
-   `max_output_tokens` (integer): The maximum number of tokens to generate for each section.

## The `sections` Array

This is a list where each item represents a single Markdown file to be generated. The order of generation is determined by the `order` field.

| Field   | Type   | Description                                                                                             |
| :------ | :----- | :------------------------------------------------------------------------------------------------------ |
| `name`  | string | A unique, internal identifier for the section (e.g., `overview`, `command-reference`).                  |
| `title` | string | The user-facing title for the section, often used as the H1 heading (e.g., "Command Reference").        |
| `order` | integer| A number that determines the sorting order of sections in the final aggregated manifest.                |
| `prompt`| string | The path to the Markdown prompt file for this section, relative to the `docs/` directory.               |
| `output`| string | The path where the generated Markdown file will be saved, relative to the `output_dir` setting.         |
| `json_key`| string | (Optional) A key used to identify this section's content when generating a structured JSON output file. |

### Per-Section Overrides

You can override the global `model` and any generation parameters on a per-section basis. This is useful for tasks that require different models or more constrained outputs.

```yaml
sections:
  - name: overview
    title: Overview
    # ...
    # This section uses the global model and settings.

  - name: command-reference
    title: Command Reference
    # ...
    # This section uses a smaller, faster model and has a higher token limit.
    model: gemini-2.0-flash-latest
    max_output_tokens: 8192
```

## Advanced Topics

### Context Management with `rules_file`

The `rules_file` setting is a key integration point with `grove-context` (`cx`). The file specified here uses a `.gitignore`-style syntax to define which source code files from your project are collected and provided to the LLM as context. A well-defined rules file is critical for generating accurate, code-aware documentation.

### JSON Schema Validation

The `docgen.config.yml` file can be validated against a JSON schema. Including the schema line at the top of your file enables autocompletion and validation in compatible editors like VS Code, helping you avoid configuration errors.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mattsolo1/grove-docgen/main/schema/docgen.config.schema.json
```