# Grove Docgen

`grove-docgen` is a command-line tool that generates technical documentation from a project's source code using LLMs and user-defined prompts. It provides a structured workflow for creating and maintaining documentation by defining it as a series of configurable sections.

<!-- placeholder for animated gif -->

### Key Features

*   **Section-Based Architecture**: Defines documentation structure in a `docgen.config.yml` file. Each section is configured with its own prompt, output file, and LLM settings.
*   **Customizable Prompts**: The `docgen init` command scaffolds a set of starter prompts in `docs/prompts/`. These files can be modified to fit the project's specific needs and documentation style.
*   **Context-Aware Generation**: Uses `grove cx generate` to build file-based context from a `.grove/rules` file, providing the LLM with relevant source code to generate accurate documentation.
*   **Interactive Customization**: The `docgen customize` command creates a `grove-flow` plan, enabling an interactive, agent-assisted workflow to refine documentation structure and content before generation.
*   **JSON Schema Enrichment**: The `docgen schema enrich` command analyzes a JSON schema, identifies properties lacking descriptions, and uses an LLM with project context to generate them.
*   **Multi-Model Support**: A global default LLM model can be set in the configuration, with the option to override it for specific sections, allowing different models to be used for different tasks.
*   **Workspace Aggregation**: The `docgen aggregate` command discovers all `docgen`-enabled packages within a workspace, generates their documentation, and collects the results into a single output directory with a `manifest.json`.
*   **README Synchronization**: The `docgen sync-readme` command generates a project `README.md` from a template, injecting content from a specified documentation section to keep the overview consistent.

## How It Works

The documentation generation process follows a repeatable pipeline:
1.  `grove-docgen` reads the `docs/docgen.config.yml` file to identify the defined documentation sections.
2.  For each section, it calls `grove cx generate` to build a file-based context based on the patterns in the configured `rules_file`.
3.  It reads the content of the section's corresponding prompt file from the `docs/prompts/` directory.
4.  It sends the generated context and the prompt to the configured LLM by calling the `grove llm request` command.
5.  The LLM's response is processed and written to the section's specified output markdown file.

## Ecosystem Integration

`grove-docgen` functions as a component of the Grove tool suite and executes other tools in the ecosystem as subprocesses.

*   **`grove cx generate`**: Provides the file-based context for all LLM requests, ensuring the model has an accurate understanding of the project's source code.
*   **`grove-flow`**: The `docgen customize` command uses `grove-flow` to create and manage an interactive, plan-based workflow, turning documentation generation into a guided, agent-assisted process.
*   **`grove llm`**: The `grove llm` facade is used to execute all requests to the configured LLM providers, handling the API interactions required to generate documentation content.

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