<!-- DOCGEN:OVERVIEW:START -->

`docgen` is a command-line tool for generating, managing, and aggregating documentation using Large Language Models (LLMs). It feeds repository context along with prompts to create accurate technical documentation, from single README sections to entire documentation websites.

## Core Mechanisms

**Section-Based Configuration**: Documentation is defined in `docgen.config.yml`. The configuration organizes documentation into logical "sections" (e.g., Overview, Usage). Each section maps a specific prompt file to an output Markdown file and can define specific LLM parameters or context rules.

**Notebook Decoupling**: Configuration and prompts can reside either in the source repository or within a `nb` workspace. This allows documentation logic and drafting to occur independently of the source code, keeping repositories clean and enabling private iteration before publishing.

**Context-Aware Generation**: The tool utilizes `grove cx` to assemble file-based context from the repository. It combines prompts, the previous documentation iteration, and `cx` codebase context to generate content via `grove llm`.

**Aggregation & Manifests**: The `aggregate` command scans a workspace for enabled packages, collects their documentation based on status (`draft`, `dev`, `production`), and generates a `manifest.json`. This output drives static site generators (like Astro) for building unified documentation portals.

## Features

### Generation & Maintenance
*   **`docgen generate`**: Generates documentation sections based on the configuration. Supports filtering by specific sections.
*   **`docgen watch`**: Monitors documentation sources and triggers incremental rebuilds on file changes. Designed for integration with hot-reloading development servers.
*   **`docgen sync-readme`**: Injects a specific generated documentation section into a `README.md.tpl` file. This ensures the repository README remains in sync with formal documentation.

### Schema Tools
*   **`docgen schema enrich`**: Parses JSON schemas and uses an LLM to generate descriptions for properties lacking them.
*   **`docgen schema generate`**: A wrapper that executes `go generate ./...` to trigger code-based schema generation.

### Workflow Management
*   **`docgen sync`**: Transfers documentation between the `grove-notebook` (drafting environment) and the local repository (version control). Supports `to-repo` and `from-repo` directions.
*   **`docgen customize`**: Generates a `grove-flow` plan to interactively customize documentation structure using AI agents.
*   **`docgen logo generate`**: Creates combined SVG assets containing a logo and text, converting text to paths to ensure consistent rendering without external font dependencies.

## Integrations

*   **`cx`**: Used to generate repository context files based on `.grove/rules`.
*   **`flow`**: Orchestrates interactive customization plans.
*   **`nb`**: Resolves workspace locations for storing prompts and drafts outside the source repository.

<!-- DOCGEN:OVERVIEW:END -->

<!-- DOCGEN:TOC:START -->

See the [documentation](docs/) for detailed usage instructions:
- [Overview](docs/01-overview.md)
- [Examples](docs/02-examples.md)
- [Configuration](docs/03-configuration.md)
- [Command Reference](docs/04-command-reference.md)

<!-- DOCGEN:TOC:END -->
