# Grove Docgen Overview

`grove-docgen` is a command-line tool for generating documentation for software projects using Large Language Models (LLMs). It integrates with the Grove ecosystem to create documentation from source code and project structure.

<!-- placeholder for animated gif -->

## Key Features

*   **Section-Based Architecture**: Documentation is defined in a `docgen.config.yml` file, organized into logical sections. Each section is configured with its own prompt, output file, and can override global settings like the LLM model.
*   **Customizable Prompts**: The `docgen init` command creates a `docs/prompts/` directory containing starter prompt files. These markdown files can be modified to direct the LLM's content generation.
*   **Interactive Customization**: The `docgen customize` command creates a `grove-flow` plan, using AI agents or structured prompts to interactively customize and generate documentation.
*   **Multi-Model Support**: The tool leverages `grove llm`, which allows the use of different LLM providers and models (e.g., Gemini, Claude, OpenAI) for different documentation sections.
*   **Workspace-Aware Context**: It uses `grove cx` to analyze project files based on configurable rules. This context is provided to the LLM during generation.
*   **README Synchronization**: A `sync-readme` command updates the project's root `README.md` by injecting content from a specified documentation section into a template file.
*   **Documentation Aggregation**: The `aggregate` command discovers all enabled packages in a workspace, copies their documentation into an output directory, and creates a `manifest.json` file.
*   **Schema Tools**: Includes commands to generate JSON schemas from Go types (`schema generate`) and enrich them with LLM-generated descriptions (`schema enrich`).

## Ecosystem Integration

`grove-docgen` functions as a component within the Grove developer tool ecosystem.

*   It uses `grove cx` to build context from source code.
*   It uses `grove llm` to send requests to language models.
*   It uses `grove-flow` to create interactive documentation plans.
*   It recognizes the workspace structure defined in the ecosystem's `grove.yml` file.
*   Prompts can be stored and resolved from a central `grove-notebook` location, with a `migrate-prompts` command to assist in moving them.

This design allows `docgen` to operate with an understanding of the project and its related components.

## How It Works

The documentation generation process consists of the following steps:

1.  **Configuration Load**: `docgen` reads the `docs/docgen.config.yml` file to determine which sections to generate.
2.  **Context Generation**: It executes `grove cx generate`, which reads project files according to patterns in a rules file and prepares context for the LLM.
3.  **Prompt Resolution**: It locates and reads the specified prompt file for the section, checking first in the associated `grove-notebook` and then falling back to the local `docs/prompts/` directory.
4.  **LLM Invocation**: It calls `grove llm request`, sending the generated context and the prompt content to the configured language model.
5.  **Output Writing**: The markdown response from the LLM is written to the section's specified output file.
6.  **JSON Regeneration (Optional)**: If configured, it parses the generated markdown files to create a structured JSON representation of the documentation.

### Installation

Install via the Grove meta-CLI:
```bash
grove install docgen
```

Verify installation:
```bash
docgen version
```

Requires the `grove` meta-CLI. See the [Grove Installation Guide](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md) if you don't have it installed.